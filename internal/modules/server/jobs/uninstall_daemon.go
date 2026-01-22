package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	"github.com/kkz6/launch-go/internal/pkg/launch/activity"
)

const TypeUninstallDaemon = "server:uninstall_daemon"

type UninstallDaemonPayload struct {
	ServerID string  `json:"server_id"`
	DaemonID string  `json:"daemon_id"`
	UserID   *string `json:"user_id,omitempty"`
}

// UninstallDaemonJob uninstalls a daemon from a server.
// Similar to Laravel's Modules\Server\Jobs\UninstallDaemon
type UninstallDaemonJob struct {
	Deps    *JobDeps
	Payload UninstallDaemonPayload

	daemon *models.Daemon
}

func NewUninstallDaemonJob(p UninstallDaemonPayload) pkgjobs.Handler {
	return &UninstallDaemonJob{Deps: deps, Payload: p}
}

// Handle processes the job
func (j *UninstallDaemonJob) Handle(ctx context.Context) error {
	var err error
	j.daemon, err = j.Deps.Repos.Daemon().FindByIDWithServer(ctx, j.Payload.DaemonID)
	if err != nil {
		return fmt.Errorf("failed to find daemon: %w", err)
	}

	// Delete daemon (stops and removes config)
	task := tasks.DeleteDaemon(tasks.DeleteDaemonConfig{
		Path:        j.daemon.Path(),
		ProgramName: j.daemon.ProgramName(),
	})

	result, err := j.Deps.RunTask(j.daemon.Server, task).
		AsRoot().
		Dispatch(ctx)

	if err != nil {
		return fmt.Errorf("failed to delete daemon: %w", err)
	}

	if !result.IsSuccessful() {
		j.Deps.Logger.Error().
			Str("output", result.GetOutput()).
			Msg("Daemon deletion task completed with errors")
	}

	// Log activity before deletion
	activity.RecordWithLogPtr(ctx, "server", "uninstalled", j.Payload.UserID, j.daemon, "Daemon was uninstalled")

	// Delete the daemon record
	if err := j.Deps.Repos.Daemon().Delete(ctx, j.daemon.ID); err != nil {
		return fmt.Errorf("failed to delete daemon record: %w", err)
	}

	j.Deps.Logger.Info().
		Str("daemon_id", j.daemon.ID).
		Str("server_id", j.daemon.ServerID).
		Msg("Daemon uninstalled successfully")

	// Broadcast event
	j.Deps.BroadcastServerEvent(j.daemon.Server, "daemon.uninstalled", map[string]any{
		"daemon_id": j.daemon.ID,
		"server_id": j.daemon.ServerID,
	})

	return nil
}

// Failed is called when the job fails after all retries
func (j *UninstallDaemonJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).
		Str("daemon_id", j.Payload.DaemonID).
		Str("server_id", j.Payload.ServerID).
		Msg("Failed to uninstall daemon")

	// Mark uninstallation as failed
	if markErr := j.Deps.Repos.Daemon().MarkUninstallationFailed(ctx, j.Payload.DaemonID); markErr != nil {
		j.Deps.Logger.Error().Err(markErr).Msg("Failed to mark daemon uninstallation as failed")
	}
}

// NewUninstallDaemonTask creates an asynq task for uninstalling a daemon
// Uses TaskID for deduplication to prevent duplicate daemon uninstallations
func NewUninstallDaemonTask(serverID, daemonID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.TaskWithID(TypeUninstallDaemon,
		UninstallDaemonPayload{ServerID: serverID, DaemonID: daemonID, UserID: userID},
		pkgjobs.Dedup("uninstall_daemon", serverID, daemonID),
	)
}
