package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/docker/tasks"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// TypeRunApplicationSchedule is the asynq task type for one tick of
// an application schedule. The asynq scheduler arms one entry per
// enabled schedule at worker boot — each tick enqueues this job,
// which resolves the current container fresh and runs the command.
//
// Mirrors dokploy's runCommand() in
// packages/server/src/utils/schedules/utils.ts:39.
const TypeRunApplicationSchedule = "docker:run_application_schedule"

// RunApplicationSchedulePayload is the cross-process payload. Only
// the schedule ID — context (application, project, server,
// container name, shell) is re-resolved on every tick so a rebuild
// that produces a new container is transparent.
type RunApplicationSchedulePayload struct {
	ScheduleID string `json:"schedule_id"`
}

// RunApplicationScheduleJob handles the asynq tick.
type RunApplicationScheduleJob struct {
	Deps    *JobDeps
	Payload RunApplicationSchedulePayload
}

// NewRunApplicationScheduleJob is the asynq constructor (see register.go).
func NewRunApplicationScheduleJob(p RunApplicationSchedulePayload) pkgjobs.Handler {
	return &RunApplicationScheduleJob{Deps: deps, Payload: p}
}

// Handle resolves the schedule + application + project + server,
// renders the run-schedule script with the current container name,
// dispatches it via taskrunner.TrackInDB() (so the resulting log
// file is reachable from <ServerLogViewer entity="task">), and
// stamps last_task_id / last_run_at / last_status on the schedule
// row.
//
// Errors are caught and persisted as status=failed — we don't want
// asynq to retry a schedule tick (next minute's tick is the natural
// retry).
func (j *RunApplicationScheduleJob) Handle(ctx context.Context) error {
	scheduleID := j.Payload.ScheduleID

	sched, err := j.Deps.Repos.Schedule().FindByID(ctx, scheduleID)
	if err != nil {
		j.Deps.Logger.Warn().Err(err).
			Str("schedule_id", scheduleID).
			Msg("schedule not found — likely deleted between scheduler tick and dispatch")
		return nil
	}
	if !sched.Enabled {
		// Schedule was disabled between scheduler tick and dispatch.
		// Drop the run.
		return nil
	}

	app, err := j.Deps.Repos.Application().FindByID(ctx, sched.ApplicationID)
	if err != nil {
		return j.persistFailure(ctx, sched.ID, fmt.Sprintf("application lookup failed: %v", err))
	}
	project, err := j.Deps.Repos.Project().FindByID(ctx, app.ProjectID)
	if err != nil {
		return j.persistFailure(ctx, sched.ID, fmt.Sprintf("project lookup failed: %v", err))
	}
	server, err := j.Deps.ServerRepos.Server().FindByID(ctx, app.ServerID)
	if err != nil {
		return j.persistFailure(ctx, sched.ID, fmt.Sprintf("server lookup failed: %v", err))
	}

	// Resolve the container name AT TICK TIME — not from a cached
	// field on the schedule row. This is the bit that makes
	// container restart / rebuild transparent: the slug is derived
	// from project+app names, so a freshly-rebuilt container with
	// the same slugs picks up the next run automatically.
	containerName := tasks.ContainerNameFor(project, app)

	script := tasks.RunScheduleScript(containerName, sched.Command, sched.ShellType)
	task := taskrunner.NewBaseTask(
		taskrunner.WithName(fmt.Sprintf("Schedule: %s", truncate(sched.Command, 40))),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(600),
	)
	result, runErr := j.Deps.RunTask(server, task).AsRoot().TrackInDB().Dispatch(ctx)

	now := time.Now().UTC()
	taskID := ""
	if result != nil && result.TaskModel != nil {
		taskID = result.TaskModel.ID
	}

	status := "success"
	if runErr != nil || (result != nil && !result.IsSuccessful()) {
		status = "failed"
	}

	updates := map[string]any{
		"last_run_at": now,
		"last_status": status,
	}
	if taskID != "" {
		updates["last_task_id"] = taskID
	}
	if err := j.Deps.Repos.Schedule().UpdateFields(ctx, sched.ID, updates); err != nil {
		j.Deps.Logger.Warn().Err(err).
			Str("schedule_id", sched.ID).
			Msg("failed to stamp schedule run metadata")
	}

	// Broadcast so the Schedules subtab refreshes the row's Last
	// Run / Status cells without polling.
	j.Deps.BroadcastToTeam(app.TeamID, "docker.application.schedule.ran", map[string]any{
		"application_id": app.ID,
		"project_id":     app.ProjectID,
		"server_id":      app.ServerID,
		"team_id":        app.TeamID,
		"schedule_id":    sched.ID,
		"status":         status,
		"task_id":        taskID,
	})

	return nil
}

// Failed is called by asynq after retries exhaust. Unlikely to fire
// for this job — Handle catches its own errors and returns nil — but
// keep it for safety so a panic in the dispatch path doesn't make
// the worker silent.
func (j *RunApplicationScheduleJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).
		Str("schedule_id", j.Payload.ScheduleID).
		Msg("application schedule run permanently failed")
}

// persistFailure records a non-task-runner failure (the SSH never
// got dispatched — application/server lookup failed, etc.) so the
// UI still shows a Last Status. Returns nil so asynq doesn't retry.
func (j *RunApplicationScheduleJob) persistFailure(ctx context.Context, scheduleID, msg string) error {
	failed := "failed"
	_ = j.Deps.Repos.Schedule().UpdateFields(ctx, scheduleID, map[string]any{
		"last_run_at": time.Now().UTC(),
		"last_status": failed,
	})
	j.Deps.Logger.Warn().
		Str("schedule_id", scheduleID).
		Str("error", msg).
		Msg("schedule run failed before dispatch")
	return nil
}

// NewRunApplicationScheduleTask packages the asynq task. The dedup
// key is the schedule ID — if a previous tick is still running when
// the next tick fires (long-running command + short cron), asynq
// silently drops the duplicate. That matches dokploy's behaviour:
// schedules don't pile up on slow runs.
func NewRunApplicationScheduleTask(scheduleID string) (*asynq.Task, error) {
	return pkgjobs.TaskWithID(TypeRunApplicationSchedule, RunApplicationSchedulePayload{
		ScheduleID: scheduleID,
	}, pkgjobs.Dedup("docker-run-schedule", scheduleID))
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
