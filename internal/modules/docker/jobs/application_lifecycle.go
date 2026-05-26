package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	dockertypes "github.com/kkz6/launch-go/internal/modules/docker/types"
	"github.com/kkz6/launch-go/internal/modules/docker/tasks"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// TypeApplicationLifecycle is the asynq task type for stop / restart /
// start against an existing application container. Rebuild is NOT
// handled here — that path enqueues a fresh deploy job with
// force_pull=true so the deploy log surface stays consistent.
const TypeApplicationLifecycle = "docker:application_lifecycle"

// ApplicationLifecyclePayload travels through asynq. ContainerName is
// resolved at enqueue time (service layer) so the worker doesn't need
// to load the project to recompute it on every run.
type ApplicationLifecyclePayload struct {
	ApplicationID string `json:"application_id"`
	ServerID      string `json:"server_id"`
	TeamID        string `json:"team_id"`
	ProjectID     string `json:"project_id"`
	// Action is one of: "stop", "restart", "start".
	Action        string `json:"action"`
	ContainerName string `json:"container_name"`
}

// ApplicationLifecycleJob handles the asynq task.
type ApplicationLifecycleJob struct {
	Deps    *JobDeps
	Payload ApplicationLifecyclePayload
}

// NewApplicationLifecycleJob is the asynq constructor (see register.go).
func NewApplicationLifecycleJob(p ApplicationLifecyclePayload) pkgjobs.Handler {
	return &ApplicationLifecycleJob{Deps: deps, Payload: p}
}

// Handle dispatches the SSH script and broadcasts terminal status.
// The frontend listens for `docker.application.updated` to refresh
// the status badge.
func (j *ApplicationLifecycleJob) Handle(ctx context.Context) error {
	server, err := j.Deps.ServerRepos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("find server: %w", err)
	}

	if j.Payload.ContainerName == "" {
		// App never deployed — running stop / restart on it is a
		// no-op. Still broadcast a terminal event so the UI clears
		// its "in progress" spinner.
		j.broadcastTerminal(j.statusForAction())
		return nil
	}

	script := tasks.ApplicationLifecycleScript(j.Payload.ContainerName, j.Payload.Action)
	task := taskrunner.NewBaseTask(
		taskrunner.WithName(fmt.Sprintf("Application %s", j.Payload.Action)),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(60),
	)
	result, runErr := j.Deps.RunTask(server, task).AsRoot().Dispatch(ctx)
	if runErr != nil {
		return runErr
	}
	if result != nil && !result.IsSuccessful() {
		return fmt.Errorf("application %s did not finish successfully", j.Payload.Action)
	}

	// Persist the new status so the General tab badge survives a page
	// reload. Status mapping mirrors what docker exposes:
	//   stop    → stopped
	//   restart → running (assume restart succeeded — if it didn't,
	//             the worker poller will reconcile to "errored").
	//   start   → running
	newStatus := j.statusForAction()
	if newStatus != "" {
		_ = j.Deps.Repos.Application().UpdateFields(ctx, j.Payload.ApplicationID, map[string]any{
			"status": newStatus,
		})
	}

	j.broadcastTerminal(newStatus)
	return nil
}

// Failed is called by asynq after retries exhaust.
func (j *ApplicationLifecycleJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).
		Str("application_id", j.Payload.ApplicationID).
		Str("container_name", j.Payload.ContainerName).
		Str("action", j.Payload.Action).
		Msg("application lifecycle action permanently failed")

	// Best-effort broadcast so the UI doesn't sit on a spinner.
	j.Deps.BroadcastToTeam(j.Payload.TeamID, "docker.application.updated", map[string]any{
		"id":             j.Payload.ApplicationID,
		"application_id": j.Payload.ApplicationID,
		"project_id":     j.Payload.ProjectID,
		"server_id":      j.Payload.ServerID,
		"team_id":        j.Payload.TeamID,
		"status":         "errored",
	})
}

func (j *ApplicationLifecycleJob) statusForAction() string {
	switch j.Payload.Action {
	case "stop":
		return string(dockertypes.ApplicationStatusStopped)
	case "restart", "start":
		return string(dockertypes.ApplicationStatusRunning)
	}
	return ""
}

func (j *ApplicationLifecycleJob) broadcastTerminal(status string) {
	j.Deps.BroadcastToTeam(j.Payload.TeamID, "docker.application.updated", map[string]any{
		"id":             j.Payload.ApplicationID,
		"application_id": j.Payload.ApplicationID,
		"project_id":     j.Payload.ProjectID,
		"server_id":      j.Payload.ServerID,
		"team_id":        j.Payload.TeamID,
		"status":         status,
	})
}

// NewApplicationLifecycleTask packages the asynq task for enqueueing
// from the service layer. Dedup key includes the action so a Stop
// queued behind a Restart isn't accidentally collapsed.
func NewApplicationLifecycleTask(
	applicationID, projectID, serverID, teamID, containerName, action string,
) (*asynq.Task, error) {
	return pkgjobs.TaskWithID(TypeApplicationLifecycle, ApplicationLifecyclePayload{
		ApplicationID: applicationID,
		ProjectID:     projectID,
		ServerID:      serverID,
		TeamID:        teamID,
		ContainerName: containerName,
		Action:        action,
	}, pkgjobs.Dedup("docker-app-lifecycle-"+action, applicationID))
}
