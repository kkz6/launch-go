package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/pkg/activity"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
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
	pkgjobs.BaseJob[*JobContext, UninstallDaemonPayload]
}

// Handle processes the job
func (j *UninstallDaemonJob) Handle(ctx context.Context) error {
	// Find the daemon with server preloaded
	daemon, err := j.Ctx.Repos.Daemon().FindByIDWithServer(ctx, j.Payload.DaemonID)
	if err != nil {
		return fmt.Errorf("failed to find daemon: %w", err)
	}

	// Delete daemon (stops and removes config)
	task := tasks.DeleteDaemon(tasks.DeleteDaemonConfig{
		Path:        daemon.Path(),
		ProgramName: daemon.ProgramName(),
	})

	result, err := j.Ctx.ForServer(daemon.Server).RunTask(task).
		AsRoot().
		Dispatch(ctx)

	if err != nil {
		return fmt.Errorf("failed to delete daemon: %w", err)
	}

	if !result.IsSuccessful() {
		j.Ctx.LogError(nil, "Daemon deletion task completed with errors",
			"output", result.GetOutput())
	}

	// Log activity before deletion
	logger := activity.New(j.Ctx.DB).
		WithContext(ctx).
		UseLog("server").
		On(daemon).
		WithEvent("uninstalled")
	if j.Payload.UserID != nil {
		logger.CausedByUser(*j.Payload.UserID)
	}
	logger.Log("Daemon was uninstalled")

	// Delete the daemon record
	if err := j.Ctx.Repos.Daemon().Delete(ctx, daemon.ID); err != nil {
		return fmt.Errorf("failed to delete daemon record: %w", err)
	}

	j.Ctx.LogInfo("Daemon uninstalled successfully",
		"daemon_id", daemon.ID,
		"server_id", daemon.ServerID,
	)

	// Broadcast event
	j.Ctx.BroadcastServerEvent(daemon.Server, "daemon.uninstalled", map[string]any{
		"daemon_id": daemon.ID,
		"server_id": daemon.ServerID,
	})

	return nil
}

// Failed is called when the job fails after all retries
func (j *UninstallDaemonJob) Failed(ctx context.Context, err error) {
	j.Ctx.LogError(err, "Failed to uninstall daemon",
		"daemon_id", j.Payload.DaemonID,
		"server_id", j.Payload.ServerID,
	)

	// Mark uninstallation as failed
	daemon, findErr := j.Ctx.Repos.Daemon().FindByID(ctx, j.Payload.DaemonID)
	if findErr == nil && daemon != nil {
		now := time.Now()
		j.Ctx.DB.Model(daemon).Updates(map[string]any{
			"uninstallation_requested_at": nil,
			"uninstallation_failed_at":    &now,
		})
	}
}

func NewUninstallDaemonJob(ctx *JobContext, payload UninstallDaemonPayload) *UninstallDaemonJob {
	return &UninstallDaemonJob{
		BaseJob: pkgjobs.NewBaseJob(ctx, payload),
	}
}

// NewUninstallDaemonTask creates an asynq task for uninstalling a daemon
// Uses TaskID for deduplication to prevent duplicate daemon uninstallations
func NewUninstallDaemonTask(serverID, daemonID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.NewTask(TypeUninstallDaemon, UninstallDaemonPayload{
		ServerID: serverID,
		DaemonID: daemonID,
		UserID:   userID,
	}, asynq.TaskID(fmt.Sprintf("uninstall_daemon:%s:%s", serverID, daemonID)))
}
