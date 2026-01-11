package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeInstallDaemon = "server:install_daemon"

// InstallDaemonPayload contains data for installing a daemon
type InstallDaemonPayload struct {
	ServerID string  `json:"server_id"`
	DaemonID string  `json:"daemon_id"`
	UserID   *string `json:"user_id,omitempty"`
}

// InstallDaemonJob handles installing a daemon on a server
type InstallDaemonJob struct {
	*JobContext
	jobs.InstallationTracker
}

// NewInstallDaemonTask creates a new asynq task for installing a daemon
func NewInstallDaemonTask(serverID, daemonID string, userID *string) (*asynq.Task, error) {
	return jobs.NewTask(TypeInstallDaemon, InstallDaemonPayload{
		ServerID: serverID,
		DaemonID: daemonID,
		UserID:   userID,
	})
}

// Handle processes the install daemon job
func (j *InstallDaemonJob) Handle(ctx context.Context, t *asynq.Task) error {
	payload, err := jobs.UnmarshalPayload[InstallDaemonPayload](t)
	if err != nil {
		return err
	}

	j.Logger.Info().
		Str("server_id", payload.ServerID).
		Str("daemon_id", payload.DaemonID).
		Msg("Installing daemon")

	// Fetch the daemon with server
	daemon, err := j.Repo.FindDaemonByIDWithServer(ctx, payload.DaemonID)
	if err != nil {
		return fmt.Errorf("failed to find daemon: %w", err)
	}

	j.broadcastProgress(payload.ServerID, "installing", fmt.Sprintf("Installing daemon: %s", daemon.Command))

	// Install the daemon configuration on the server
	_, err = j.RunTask(daemon.Server, tasks.NewInstallDaemon(daemon.Server, daemon)).
		AsRoot().
		Throw().
		Dispatch(ctx)
	if err != nil {
		return fmt.Errorf("failed to install daemon config: %w", err)
	}

	// Reload supervisor to pick up the new configuration
	_, err = j.RunTask(daemon.Server, tasks.NewReloadSupervisor()).
		AsRoot().
		Throw().
		Dispatch(ctx)
	if err != nil {
		return fmt.Errorf("failed to reload supervisor: %w", err)
	}

	// Mark the daemon as installed
	if err := j.Repo.MarkDaemonInstalled(ctx, daemon.ID); err != nil {
		return fmt.Errorf("failed to update daemon status: %w", err)
	}

	j.broadcastProgress(payload.ServerID, "installed", "Daemon installed successfully")

	return nil
}

// Failed handles job failure
func (j *InstallDaemonJob) Failed(ctx context.Context, t *asynq.Task, err error) {
	payload, unmarshalErr := jobs.UnmarshalPayload[InstallDaemonPayload](t)
	if unmarshalErr != nil {
		j.Logger.Error().Err(unmarshalErr).Msg("Failed to unmarshal payload in failure handler")
		return
	}

	j.Logger.Error().
		Err(err).
		Str("server_id", payload.ServerID).
		Str("daemon_id", payload.DaemonID).
		Msg("Failed to install daemon")

	// Fetch the daemon for updating status
	daemon, findErr := j.Repo.FindDaemonByID(ctx, payload.DaemonID)
	if findErr != nil {
		j.broadcastProgress(payload.ServerID, "failed", "Failed to install daemon")
		return
	}

	// Mark installation as failed
	j.MarkInstallationFailed(j.DB, daemon)

	j.broadcastProgress(payload.ServerID, "failed", fmt.Sprintf("Failed to install daemon: %s", daemon.Command))
}

func (j *InstallDaemonJob) broadcastProgress(serverID, status, message string) {
	j.BroadcastToServer(serverID, "server.daemon.progress", map[string]interface{}{
		"server_id": serverID,
		"status":    status,
		"message":   message,
	})
}
