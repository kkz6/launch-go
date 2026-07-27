package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	"github.com/kkz6/launch-go/internal/pkg/launch/paths"
)

const TypeFetchTaskOutput = "server:fetch_task_output"

// FetchTaskOutputPayload holds data for fetching task output from server
type FetchTaskOutputPayload struct {
	TaskID                string `json:"task_id"`
	ServerID              string `json:"server_id"`
	RescheduleIntervalSec int    `json:"reschedule_interval_sec"` // 0 = no reschedule
}

// FetchTaskOutputJob SSHes into the server to download task output.
// If RescheduleIntervalSec > 0 and task is still pending, it reschedules itself.
type FetchTaskOutputJob struct {
	Deps    *JobDeps
	Payload FetchTaskOutputPayload

	task   *models.Task
	server *models.Server
}

func NewFetchTaskOutputJob(p FetchTaskOutputPayload) pkgjobs.Handler {
	return &FetchTaskOutputJob{Deps: deps, Payload: p}
}

// Handle executes the fetch task output job
func (j *FetchTaskOutputJob) Handle(ctx context.Context) error {
	var err error

	// Load task
	j.task, err = j.Deps.Repos.Task().FindByID(ctx, j.Payload.TaskID)
	if err != nil {
		return fmt.Errorf("failed to find task: %w", err)
	}

	// Load server
	j.server, err = j.Deps.Repos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	j.Deps.Logger.Info().
		Str("task_id", j.task.ID).
		Str("server_id", j.server.ID).
		Msg("Fetching task output from server")

	// Build output file path
	taskDir := paths.GetTaskDir(j.task.User)
	taskPaths := paths.GetTaskPaths(taskDir, j.task.ID)

	// Create GetFile task to fetch the output
	getFileTask := tasks.GetFile(tasks.GetFileConfig{
		Path:     taskPaths.Output,
		MaxBytes: 1024 * 1024, // 1MB
	})

	// Run as root to ensure we can read the file
	result, err := j.Deps.RunTask(j.server, getFileTask).
		WithoutTracking().
		AsRoot().
		Run(ctx)

	if err != nil {
		j.Deps.Logger.Error().Err(err).
			Str("task_id", j.task.ID).
			Msg("Failed to fetch task output")
		return err
	}

	if result.TaskResult != nil && result.TaskResult.IsSuccessful() {
		// Update task output in database
		output := result.TaskResult.Output
		if err := j.Deps.Repos.Task().UpdateOutput(ctx, j.task.ID, output); err != nil {
			j.Deps.Logger.Error().Err(err).
				Str("task_id", j.task.ID).
				Msg("Failed to update task output")
		} else {
			j.Deps.Logger.Info().
				Str("task_id", j.task.ID).
				Msg("Task output updated")
		}

		// Broadcast output update
		j.Deps.BroadcastServerEvent(j.server, "task.output", map[string]any{
			"task_id": j.task.ID,
			"output":  output,
			"status":  j.task.Status,
		})
	}

	// Check if task has timed out (status still pending but exceeded timeout)
	if j.task.Status == "pending" || j.task.Status == "running" {
		if j.task.CreatedAt != nil {
			timeoutDuration := time.Duration(j.task.Timeout) * time.Second
			if time.Since(*j.task.CreatedAt) > timeoutDuration {
				exitCode := 124
				if err := j.Deps.Repos.Task().UpdateFields(ctx, j.task.ID, map[string]any{
					"status":    "timeout",
					"exit_code": exitCode,
				}); err != nil {
					j.Deps.Logger.Error().Err(err).
						Str("task_id", j.task.ID).
						Msg("Failed to mark task as timeout")
				} else {
					j.Deps.Logger.Info().
						Str("task_id", j.task.ID).
						Msg("Task marked as timeout")
				}
			}
		}
	}

	// Self-reschedule if interval > 0 and task is still running
	if j.Payload.RescheduleIntervalSec > 0 {
		// Reload task to check current status
		refreshedTask, findErr := j.Deps.Repos.Task().FindByID(ctx, j.Payload.TaskID)
		if findErr != nil {
			j.Deps.Logger.Warn().Err(findErr).
				Str("task_id", j.Payload.TaskID).
				Msg("Failed to reload task before rescheduling output fetch")
			return nil
		}
		j.task = refreshedTask
		if j.task != nil && (j.task.Status == "pending" || j.task.Status == "running") {
			j.reschedule()
		}
	}

	return nil
}

// reschedule dispatches a new job with the same parameters after the interval
func (j *FetchTaskOutputJob) reschedule() {
	if j.Deps.Queue == nil {
		return
	}

	newTask, err := NewFetchTaskOutputTask(
		j.Payload.TaskID,
		j.Payload.ServerID,
		j.Payload.RescheduleIntervalSec,
	)
	if err != nil {
		j.Deps.Logger.Error().Err(err).Msg("Failed to create reschedule task")
		return
	}

	delay := time.Duration(j.Payload.RescheduleIntervalSec) * time.Second
	_, err = j.Deps.Queue.Enqueue(newTask, asynq.ProcessIn(delay))
	if err != nil {
		j.Deps.Logger.Error().Err(err).Msg("Failed to reschedule fetch task output job")
	} else {
		j.Deps.Logger.Info().
			Str("task_id", j.Payload.TaskID).
			Int("delay_seconds", j.Payload.RescheduleIntervalSec).
			Msg("Rescheduled fetch task output job")
	}
}

// Failed handles job failure
func (j *FetchTaskOutputJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).
		Str("task_id", j.Payload.TaskID).
		Msg("Failed to fetch task output")
}

// NewFetchTaskOutputTask creates a fetch task output asynq task
func NewFetchTaskOutputTask(taskID, serverID string, rescheduleIntervalSec int) (*asynq.Task, error) {
	return pkgjobs.Task(TypeFetchTaskOutput, FetchTaskOutputPayload{
		TaskID:                taskID,
		ServerID:              serverID,
		RescheduleIntervalSec: rescheduleIntervalSec,
	})
}
