package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/pkg/jobs"
)

// UninstallDaemonJob uninstalls a daemon from a server.
// Similar to Laravel's Modules\Server\Jobs\UninstallDaemon
type UninstallDaemonJob struct {
	ServerJobBase
	jobs.UninstallationTracker
	Payload UninstallDaemonPayload
}

// Type returns the job type identifier
func (j *UninstallDaemonJob) Type() string {
	return TypeUninstallDaemon
}

// Handle processes the job
func (j *UninstallDaemonJob) Handle(ctx context.Context) error {
	// Find the daemon with server preloaded
	daemon, err := j.Repo().FindDaemonByIDWithServer(ctx, j.Payload.DaemonID)
	if err != nil {
		return fmt.Errorf("failed to find daemon: %w", err)
	}

	// Delete daemon (stops and removes config)
	task := tasks.DeleteDaemon(tasks.DeleteDaemonConfig{
		Path:        daemon.Path(),
		ProgramName: daemon.ProgramName(),
	})

	result, err := j.RunTaskOnServer(daemon.Server, task).
		AsRoot().
		Dispatch(ctx)

	if err != nil {
		return fmt.Errorf("failed to delete daemon: %w", err)
	}

	if !result.IsSuccessful() {
		j.LogError(nil, "Daemon deletion task completed with errors",
			"output", result.GetOutput())
	}

	// Delete the daemon record
	if err := j.Repo().DeleteDaemon(ctx, daemon.ID); err != nil {
		return fmt.Errorf("failed to delete daemon record: %w", err)
	}

	j.LogInfo("Daemon uninstalled successfully",
		"daemon_id", daemon.ID,
		"server_id", daemon.ServerID,
	)

	// Broadcast event
	j.BroadcastServerEvent(daemon.ServerID, "daemon.uninstalled", map[string]any{
		"daemon_id": daemon.ID,
		"server_id": daemon.ServerID,
	})

	return nil
}

// Failed is called when the job fails after all retries
func (j *UninstallDaemonJob) Failed(ctx context.Context, err error) {
	j.LogError(err, "Failed to uninstall daemon",
		"daemon_id", j.Payload.DaemonID,
		"server_id", j.Payload.ServerID,
	)

	// Mark uninstallation as failed
	daemon, findErr := j.Repo().FindDaemonByID(ctx, j.Payload.DaemonID)
	if findErr == nil && daemon != nil {
		j.MarkUninstallationFailed(j.DB, daemon)
	}
}

// NewUninstallDaemonTask creates an asynq task for uninstalling a daemon
func NewUninstallDaemonTask(serverID, daemonID string, userID *string) (*asynq.Task, error) {
	return jobs.NewTask(TypeUninstallDaemon, UninstallDaemonPayload{
		ServerID: serverID,
		DaemonID: daemonID,
		UserID:   userID,
	})
}
