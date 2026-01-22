package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeRestartDaemon = "server:restart_daemon"

type RestartDaemonPayload struct {
	ServerID string  `json:"server_id"`
	DaemonID string  `json:"daemon_id"`
	UserID   *string `json:"user_id,omitempty"`
}

// RestartDaemonJob restarts a daemon on a server.
// Similar to Laravel's Modules\Server\Jobs\RestartDaemon
type RestartDaemonJob struct {
	Deps    *JobDeps
	Payload RestartDaemonPayload

	daemon *models.Daemon
}

func NewRestartDaemonJob(p RestartDaemonPayload) pkgjobs.Handler {
	return &RestartDaemonJob{Deps: deps, Payload: p}
}

// Handle processes the job
func (j *RestartDaemonJob) Handle(ctx context.Context) error {
	var err error

	// Find the daemon with server preloaded
	j.daemon, err = j.Deps.Repos.Daemon().FindByIDWithServer(ctx, j.Payload.DaemonID)
	if err != nil {
		return fmt.Errorf("failed to find daemon: %w", err)
	}

	// Restart the daemon
	task := tasks.RestartDaemon(tasks.RestartDaemonConfig{
		ProgramName: j.daemon.ProgramName(),
	})

	result, err := j.Deps.RunTask(j.daemon.Server, task).
		AsRoot().
		Dispatch(ctx)

	if err != nil {
		return fmt.Errorf("failed to restart daemon: %w", err)
	}

	if !result.IsSuccessful() {
		return fmt.Errorf("failed to restart daemon: %s", result.GetOutput())
	}

	j.Deps.Logger.Info().
		Str("daemon_id", j.daemon.ID).
		Str("server_id", j.daemon.ServerID).
		Msg("daemon restarted successfully")

	// Broadcast event
	j.Deps.BroadcastServerEvent(j.daemon.Server, "daemon.restarted", map[string]any{
		"daemon_id": j.daemon.ID,
		"server_id": j.daemon.ServerID,
	})

	return nil
}

// Failed is called when the job fails after all retries
func (j *RestartDaemonJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).
		Str("daemon_id", j.Payload.DaemonID).
		Str("server_id", j.Payload.ServerID).
		Msg("failed to restart daemon")
}

// NewRestartDaemonTask creates an asynq task for restarting a daemon
// Uses TaskID for deduplication to prevent duplicate daemon restarts
func NewRestartDaemonTask(serverID, daemonID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.TaskWithID(TypeRestartDaemon,
		RestartDaemonPayload{
			ServerID: serverID,
			DaemonID: daemonID,
			UserID:   userID,
		},
		pkgjobs.Dedup("restart_daemon", serverID, daemonID),
	)
}
