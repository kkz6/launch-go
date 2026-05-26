package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/docker/tasks"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// TypeRemoveApplication is the asynq task type for tearing down the
// container belonging to a docker application after the row has been
// soft-deleted from the DB.
//
// We make this a background job rather than a synchronous call from
// DeleteApplication because (a) docker stop can take 10+ seconds on
// large images, (b) the user expects the row to disappear from the
// UI instantly, and (c) if the SSH call fails we want it to retry
// instead of leaving the row gone and the container still running.
const TypeRemoveApplication = "docker:remove_application"

// RemoveApplicationPayload travels through asynq. We carry both IDs
// AND the resolved container name because the application row is
// already soft-deleted by the time this runs — loading it back via
// Unscoped queries everywhere is leakier than just sending the name.
type RemoveApplicationPayload struct {
	ApplicationID string `json:"application_id"`
	ServerID      string `json:"server_id"`
	TeamID        string `json:"team_id"`
	ProjectID     string `json:"project_id"`
	// ContainerName is the on-host name computed at delete time —
	// `launch-app-<project-slug>-<app-slug>`. Caller passes it so the
	// job doesn't need to re-resolve the project on a deleted row.
	ContainerName string `json:"container_name"`
	// VolumeNames is the list of docker named volumes the application
	// declared (kind=volume rows on docker_application_volumes). When
	// non-empty, the remove script runs `docker volume rm` for each
	// after the container is gone. Caller resolves the names from the
	// repo before soft-deleting the rows. Empty slice = keep all
	// volumes (the default opt-out behaviour).
	VolumeNames []string `json:"volume_names,omitempty"`
}

// RemoveApplicationJob is the handler.
type RemoveApplicationJob struct {
	Deps    *JobDeps
	Payload RemoveApplicationPayload
}

// NewRemoveApplicationJob is the asynq constructor (see register.go).
func NewRemoveApplicationJob(p RemoveApplicationPayload) pkgjobs.Handler {
	return &RemoveApplicationJob{Deps: deps, Payload: p}
}

// Handle runs the docker stop + rm script. Any failure here means the
// container is still on the host — we log it but don't keep retrying
// indefinitely; asynq's default retry policy gives us 25 attempts
// over a few hours which is enough to ride out a transient SSH
// failure without spamming a permanently broken host.
func (j *RemoveApplicationJob) Handle(ctx context.Context) error {
	server, err := j.Deps.ServerRepos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("find server: %w", err)
	}

	if j.Payload.ContainerName == "" {
		// Nothing to do — the app never deployed. Just broadcast so
		// any UI listening for the terminal event resolves its
		// spinner.
		j.Deps.BroadcastToTeam(j.Payload.TeamID, "docker.application.removed", map[string]any{
			"application_id":  j.Payload.ApplicationID,
			"project_id":      j.Payload.ProjectID,
			"server_id":       j.Payload.ServerID,
			"team_id":         j.Payload.TeamID,
			"container_found": false,
		})
		return nil
	}

	task := taskrunner.NewBaseTask(
		taskrunner.WithName("Remove application container"),
		taskrunner.WithScript(tasks.RemoveApplicationScript(j.Payload.ContainerName, j.Payload.VolumeNames)),
		taskrunner.WithTimeoutSeconds(60),
	)
	result, runErr := j.Deps.RunTask(server, task).AsRoot().Dispatch(ctx)
	if runErr != nil {
		// Treat genuine SSH errors as transient and let asynq retry.
		return runErr
	}
	if result != nil && !result.IsSuccessful() {
		// Non-zero exit on container-already-gone is benign — the
		// script itself tolerates a missing container, so a hard
		// failure here means something unexpected. Log loudly but
		// don't return — asynq retry won't help.
		j.Deps.Logger.Warn().
			Str("application_id", j.Payload.ApplicationID).
			Str("container_name", j.Payload.ContainerName).
			Msg("application removal exited non-zero; container may need manual cleanup")
	}

	j.Deps.BroadcastToTeam(j.Payload.TeamID, "docker.application.removed", map[string]any{
		"application_id":  j.Payload.ApplicationID,
		"project_id":      j.Payload.ProjectID,
		"server_id":       j.Payload.ServerID,
		"team_id":         j.Payload.TeamID,
		"container_found": true,
	})
	return nil
}

// Failed is called by asynq after retries exhaust. Log loudly — this
// is an operator-actionable failure (container left behind).
func (j *RemoveApplicationJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).
		Str("application_id", j.Payload.ApplicationID).
		Str("container_name", j.Payload.ContainerName).
		Msg("application removal permanently failed — container may still be running")
}

// NewRemoveApplicationTask packages the asynq task for enqueueing
// from the service layer.
func NewRemoveApplicationTask(
	applicationID, projectID, serverID, teamID, containerName string,
	volumeNames []string,
) (*asynq.Task, error) {
	return pkgjobs.TaskWithID(TypeRemoveApplication, RemoveApplicationPayload{
		ApplicationID: applicationID,
		ProjectID:     projectID,
		ServerID:      serverID,
		TeamID:        teamID,
		ContainerName: containerName,
		VolumeNames:   volumeNames,
	}, pkgjobs.Dedup("docker-remove-app", applicationID))
}
