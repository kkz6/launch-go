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

// RemoveApplicationPayload travels through asynq. The row is no
// longer soft-deleted up-front — it stays visible with
// status="deleting" until this job confirms the container is gone.
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
	// PreviousStatus is the application's status BEFORE the user
	// clicked Delete. Carried so the job can revert the row out of
	// the "deleting" placeholder back to its real prior state on
	// any failure path (SSH error, docker stop fails, asynq retries
	// exhaust). User chose "keep row, revert to old status" over
	// "keep row, flip to failed" in the design review.
	PreviousStatus string `json:"previous_status,omitempty"`
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

// Handle runs the docker stop + rm script then soft-deletes the
// application row. The row stays visible with status="deleting"
// until this job either:
//
//   - succeeds → soft-delete row + broadcast docker.application.deleted
//   - fails    → revert status to PreviousStatus + broadcast
//     docker.application.updated (row stays usable)
//
// Any returned error puts the job back in asynq's retry queue;
// asynq's default policy is ~25 attempts over a few hours, which
// rides out transient SSH blips. Once retries exhaust, the Failed
// hook below runs the revert path so the row never stays stuck.
func (j *RemoveApplicationJob) Handle(ctx context.Context) error {
	server, err := j.Deps.ServerRepos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("find server: %w", err)
	}

	if j.Payload.ContainerName == "" {
		// App never deployed — no container to stop. Go straight
		// to the success/finalise path.
		return j.finaliseDelete(ctx, false)
	}

	task := taskrunner.NewBaseTask(
		taskrunner.WithName("Remove application container"),
		taskrunner.WithScript(tasks.RemoveApplicationScript(j.Payload.ContainerName, j.Payload.VolumeNames)),
		taskrunner.WithTimeoutSeconds(60),
	)
	result, runErr := j.Deps.RunTask(server, task).AsRoot().Dispatch(ctx)
	if runErr != nil {
		// Genuine SSH errors → return so asynq retries. The Failed
		// hook below will run the revert path once retries exhaust.
		return runErr
	}
	if result != nil && !result.IsSuccessful() {
		// Script itself reports failure (rare — the teardown script
		// tolerates a missing container). Log; don't retry because
		// the next attempt would hit the same outcome.
		j.Deps.Logger.Warn().
			Str("application_id", j.Payload.ApplicationID).
			Str("container_name", j.Payload.ContainerName).
			Msg("application removal exited non-zero; container may need manual cleanup")
	}
	return j.finaliseDelete(ctx, true)
}

// finaliseDelete soft-deletes the row and broadcasts the terminal
// docker.application.deleted event. Called from the success path of
// Handle once docker stop + rm have run (or been skipped because the
// app never deployed).
func (j *RemoveApplicationJob) finaliseDelete(ctx context.Context, containerFound bool) error {
	if err := j.Deps.Repos.Application().Delete(ctx, j.Payload.ApplicationID); err != nil {
		// DB write failed AFTER the container is already gone —
		// surface as a job error so asynq retries the row deletion.
		// The row will sit in "deleting" until the retry lands.
		return fmt.Errorf("soft-delete application row: %w", err)
	}
	j.Deps.BroadcastToTeam(j.Payload.TeamID, "docker.application.deleted", map[string]any{
		"id":              j.Payload.ApplicationID,
		"application_id":  j.Payload.ApplicationID,
		"project_id":      j.Payload.ProjectID,
		"server_id":       j.Payload.ServerID,
		"team_id":         j.Payload.TeamID,
		"container_found": containerFound,
	})
	return nil
}

// Failed is called by asynq after retries exhaust. Revert the row
// out of "deleting" so the operator can see it again, decide what
// to do, and try again — never leave the row stranded in the
// in-flight placeholder state.
func (j *RemoveApplicationJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).
		Str("application_id", j.Payload.ApplicationID).
		Str("container_name", j.Payload.ContainerName).
		Msg("application removal permanently failed — reverting status and leaving the row in place")

	prev := j.Payload.PreviousStatus
	if prev == "" {
		// Defensive — if an older job payload predates the field,
		// fall back to "failed" rather than leaving the row stuck
		// in "deleting".
		prev = "failed"
	}
	if updateErr := j.Deps.Repos.Application().UpdateStatus(ctx, j.Payload.ApplicationID, prev); updateErr != nil {
		j.Deps.Logger.Error().Err(updateErr).
			Str("application_id", j.Payload.ApplicationID).
			Msg("could not revert application status after delete failure — row may be stuck on 'deleting'")
	}
	j.Deps.BroadcastToTeam(j.Payload.TeamID, "docker.application.updated", map[string]any{
		"id":         j.Payload.ApplicationID,
		"project_id": j.Payload.ProjectID,
		"server_id":  j.Payload.ServerID,
		"team_id":    j.Payload.TeamID,
		"status":     prev,
	})
}

// NewRemoveApplicationTask packages the asynq task for enqueueing
// from the service layer. PreviousStatus is what the row's status
// was just before the user clicked Delete — used by the job's
// Failed hook to revert the row out of the "deleting" placeholder.
func NewRemoveApplicationTask(
	applicationID, projectID, serverID, teamID, containerName string,
	volumeNames []string,
	previousStatus string,
) (*asynq.Task, error) {
	return pkgjobs.TaskWithID(TypeRemoveApplication, RemoveApplicationPayload{
		ApplicationID:  applicationID,
		ProjectID:      projectID,
		ServerID:       serverID,
		TeamID:         teamID,
		ContainerName:  containerName,
		VolumeNames:    volumeNames,
		PreviousStatus: previousStatus,
	}, pkgjobs.Dedup("docker-remove-app", applicationID))
}
