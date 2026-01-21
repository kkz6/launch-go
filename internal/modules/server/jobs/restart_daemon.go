package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

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
	pkgjobs.BaseJob[*JobContext, RestartDaemonPayload]
}

// Handle processes the job
func (j *RestartDaemonJob) Handle(ctx context.Context) error {
	// Find the daemon with server preloaded
	daemon, err := j.Ctx.Repos.Daemon().FindByIDWithServer(ctx, j.Payload.DaemonID)
	if err != nil {
		return fmt.Errorf("failed to find daemon: %w", err)
	}

	// Restart the daemon
	task := tasks.RestartDaemon(tasks.RestartDaemonConfig{
		ProgramName: daemon.ProgramName(),
	})

	result, err := j.Ctx.ForServer(daemon.Server).RunTask(task).
		AsRoot().
		Dispatch(ctx)

	if err != nil {
		return fmt.Errorf("failed to restart daemon: %w", err)
	}

	if !result.IsSuccessful() {
		return fmt.Errorf("failed to restart daemon: %s", result.GetOutput())
	}

	j.Ctx.LogInfo("Daemon restarted successfully",
		"daemon_id", daemon.ID,
		"server_id", daemon.ServerID,
	)

	// Broadcast event
	j.Ctx.BroadcastServerEvent(daemon.Server, "daemon.restarted", map[string]any{
		"daemon_id": daemon.ID,
		"server_id": daemon.ServerID,
	})

	return nil
}

// Failed is called when the job fails after all retries
func (j *RestartDaemonJob) Failed(ctx context.Context, err error) {
	j.Ctx.LogError(err, "Failed to restart daemon",
		"daemon_id", j.Payload.DaemonID,
		"server_id", j.Payload.ServerID,
	)
}

func NewRestartDaemonJob(ctx *JobContext, payload RestartDaemonPayload) *RestartDaemonJob {
	return &RestartDaemonJob{
		BaseJob: pkgjobs.NewBaseJob(ctx, payload),
	}
}

// NewRestartDaemonTask creates an asynq task for restarting a daemon
// Uses TaskID for deduplication to prevent duplicate daemon restarts
func NewRestartDaemonTask(serverID, daemonID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.NewTask(TypeRestartDaemon, RestartDaemonPayload{
		ServerID: serverID,
		DaemonID: daemonID,
		UserID:   userID,
	}, asynq.TaskID(fmt.Sprintf("restart_daemon:%s:%s", serverID, daemonID)))
}
