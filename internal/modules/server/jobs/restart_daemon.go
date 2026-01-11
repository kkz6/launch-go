package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeRestartDaemon = "server:restart_daemon"

// RestartDaemonPayload contains data for restarting a daemon
type RestartDaemonPayload struct {
	ServerID string `json:"server_id"`
	DaemonID string `json:"daemon_id"`
}

// RestartDaemonJob handles restarting a daemon on a server
type RestartDaemonJob struct {
	*JobContext
}

// NewRestartDaemonTask creates a new asynq task for restarting a daemon
func NewRestartDaemonTask(serverID, daemonID string) (*asynq.Task, error) {
	return jobs.NewTask(TypeRestartDaemon, RestartDaemonPayload{
		ServerID: serverID,
		DaemonID: daemonID,
	})
}

// Handle processes the restart daemon job
func (j *RestartDaemonJob) Handle(ctx context.Context, t *asynq.Task) error {
	payload, err := jobs.UnmarshalPayload[RestartDaemonPayload](t)
	if err != nil {
		return err
	}

	j.Logger.Info().
		Str("server_id", payload.ServerID).
		Str("daemon_id", payload.DaemonID).
		Msg("Restarting daemon")

	// Fetch the server
	server, err := j.FindServer(ctx, payload.ServerID)
	if err != nil {
		return err
	}

	// Fetch the daemon
	daemon, err := j.Repo.FindDaemonByID(ctx, payload.DaemonID)
	if err != nil {
		return fmt.Errorf("failed to find daemon: %w", err)
	}

	j.broadcastProgress(payload.ServerID, "restarting", fmt.Sprintf("Restarting daemon: %s", daemon.Command))

	// TODO: Run the actual daemon restart task
	// _, err = j.RunTask(server, tasks.NewRestartDaemon(&daemon)).
	//     AsRoot().
	//     Dispatch(ctx)
	// if err != nil {
	//     return err
	// }

	_ = server // use server when implementing task execution

	j.broadcastProgress(payload.ServerID, "running", "Daemon restarted successfully")

	return nil
}

// Failed handles job failure
func (j *RestartDaemonJob) Failed(ctx context.Context, t *asynq.Task, err error) {
	payload, unmarshalErr := jobs.UnmarshalPayload[RestartDaemonPayload](t)
	if unmarshalErr != nil {
		j.Logger.Error().Err(unmarshalErr).Msg("Failed to unmarshal payload in failure handler")
		return
	}

	j.Logger.Error().
		Err(err).
		Str("server_id", payload.ServerID).
		Str("daemon_id", payload.DaemonID).
		Msg("Failed to restart daemon")

	j.broadcastProgress(payload.ServerID, "failed", "Failed to restart daemon")
}

func (j *RestartDaemonJob) broadcastProgress(serverID, status, message string) {
	j.BroadcastToServer(serverID, "server.daemon.progress", map[string]interface{}{
		"server_id": serverID,
		"status":    status,
		"message":   message,
	})
}
