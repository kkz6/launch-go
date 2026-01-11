package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeUninstallDaemon = "server:uninstall_daemon"

// UninstallDaemonPayload contains data for uninstalling a daemon
type UninstallDaemonPayload struct {
	ServerID string  `json:"server_id"`
	DaemonID string  `json:"daemon_id"`
	UserID   *string `json:"user_id,omitempty"`
}

// UninstallDaemonJob handles uninstalling a daemon from a server
type UninstallDaemonJob struct {
	*JobContext
	jobs.UninstallationTracker
}

// NewUninstallDaemonTask creates a new asynq task for uninstalling a daemon
func NewUninstallDaemonTask(serverID, daemonID string, userID *string) (*asynq.Task, error) {
	return jobs.NewTask(TypeUninstallDaemon, UninstallDaemonPayload{
		ServerID: serverID,
		DaemonID: daemonID,
		UserID:   userID,
	})
}

// Handle processes the uninstall daemon job
func (j *UninstallDaemonJob) Handle(ctx context.Context, t *asynq.Task) error {
	payload, err := jobs.UnmarshalPayload[UninstallDaemonPayload](t)
	if err != nil {
		return err
	}

	j.Logger.Info().
		Str("server_id", payload.ServerID).
		Str("daemon_id", payload.DaemonID).
		Msg("Uninstalling daemon")

	// Fetch the daemon with server
	daemon, err := j.Repo.FindDaemonByIDWithServer(ctx, payload.DaemonID)
	if err != nil {
		return fmt.Errorf("failed to find daemon: %w", err)
	}

	j.broadcastProgress(payload.ServerID, payload.DaemonID, "uninstalling", fmt.Sprintf("Uninstalling daemon: %s", daemon.Command))

	// TODO: Run the actual daemon uninstallation task
	// _, err = j.RunTask(daemon.Server, tasks.NewUninstallDaemon(&daemon)).
	//     AsRoot().
	//     Dispatch(ctx)
	// if err != nil {
	//     return err
	// }

	// Delete the daemon record
	if err := j.Repo.DeleteDaemon(ctx, daemon.ID); err != nil {
		return fmt.Errorf("failed to delete daemon record: %w", err)
	}

	j.broadcastProgress(payload.ServerID, payload.DaemonID, "uninstalled", "Daemon uninstalled successfully")

	return nil
}

// Failed handles job failure
func (j *UninstallDaemonJob) Failed(ctx context.Context, t *asynq.Task, err error) {
	payload, unmarshalErr := jobs.UnmarshalPayload[UninstallDaemonPayload](t)
	if unmarshalErr != nil {
		j.Logger.Error().Err(unmarshalErr).Msg("Failed to unmarshal payload in failure handler")
		return
	}

	j.Logger.Error().
		Err(err).
		Str("server_id", payload.ServerID).
		Str("daemon_id", payload.DaemonID).
		Msg("Failed to uninstall daemon")

	// Fetch the daemon for error message
	daemon, findErr := j.Repo.FindDaemonByID(ctx, payload.DaemonID)
	if findErr != nil {
		j.broadcastProgress(payload.ServerID, payload.DaemonID, "failed", "Failed to uninstall daemon")
		return
	}

	j.broadcastProgress(payload.ServerID, payload.DaemonID, "failed", fmt.Sprintf("Failed to uninstall daemon: %s", daemon.Command))
}

func (j *UninstallDaemonJob) broadcastProgress(serverID, daemonID, status, message string) {
	j.BroadcastToServer(serverID, "server.daemon.progress", map[string]interface{}{
		"server_id": serverID,
		"daemon_id": daemonID,
		"status":    status,
		"message":   message,
		"timestamp": time.Now().Format(time.RFC3339),
	})
}
