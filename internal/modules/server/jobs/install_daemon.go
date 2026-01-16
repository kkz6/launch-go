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

const TypeInstallDaemon = "server:install_daemon"

type InstallDaemonPayload struct {
	ServerID string  `json:"server_id"`
	DaemonID string  `json:"daemon_id"`
	UserID   *string `json:"user_id,omitempty"`
}

// InstallDaemonJob installs a daemon (supervisor program) on a server.
type InstallDaemonJob struct {
	ctx     *JobContext
	Payload InstallDaemonPayload
}

func (j *InstallDaemonJob) Handle(ctx context.Context) error {
	daemon, err := j.ctx.Repos.Daemon().FindByIDWithServer(ctx, j.Payload.DaemonID)
	if err != nil {
		return fmt.Errorf("failed to find daemon: %w", err)
	}

	contents := daemon.ToSupervisorConfig()

	uploadTask := tasks.UploadDaemon(tasks.UploadDaemonConfig{
		Path:         daemon.Path(),
		Contents:     contents,
		LogPath:      daemon.GetLogPath(),
		ErrorLogPath: daemon.GetErrorLogPath(),
		User:         daemon.User,
	})

	result, err := j.ctx.ForServer(daemon.Server).RunTask(uploadTask).
		AsRoot().
		Dispatch(ctx)

	if err != nil {
		return fmt.Errorf("failed to upload daemon config: %w", err)
	}

	if !result.IsSuccessful() {
		return fmt.Errorf("failed to upload daemon config: %s", result.GetOutput())
	}

	reloadTask := tasks.ReloadSupervisor()
	_, err = j.ctx.ForServer(daemon.Server).RunTask(reloadTask).
		AsRoot().
		Dispatch(ctx)

	if err != nil {
		j.ctx.LogError(err, "Failed to reload supervisor, daemon may not start")
	}

	if err := j.ctx.Repos.Daemon().MarkInstalled(ctx, daemon.ID); err != nil {
		return fmt.Errorf("failed to mark daemon as installed: %w", err)
	}

	// Log activity
	logger := activity.New(j.ctx.DB).
		WithContext(ctx).
		UseLog("server").
		On(daemon).
		WithEvent("installed")
	if j.Payload.UserID != nil {
		logger.CausedByUser(*j.Payload.UserID)
	}
	logger.Log("Daemon was installed")

	j.ctx.LogInfo("Daemon installed successfully",
		"daemon_id", daemon.ID,
		"server_id", daemon.ServerID,
		"command", daemon.Command,
	)

	j.ctx.BroadcastServerEvent(daemon.Server, "daemon.installed", map[string]any{
		"daemon_id": daemon.ID,
		"server_id": daemon.ServerID,
	})

	return nil
}

func (j *InstallDaemonJob) Failed(ctx context.Context, err error) {
	j.ctx.LogError(err, "Failed to install daemon",
		"daemon_id", j.Payload.DaemonID,
		"server_id", j.Payload.ServerID,
	)

	daemon, findErr := j.ctx.Repos.Daemon().FindByID(ctx, j.Payload.DaemonID)
	if findErr == nil && daemon != nil {
		now := time.Now()
		j.ctx.DB.Model(daemon).Updates(map[string]any{
			"installed_at":           nil,
			"installation_failed_at": &now,
		})
	}
}

func NewInstallDaemonJob(ctx *JobContext, payload InstallDaemonPayload) *InstallDaemonJob {
	return &InstallDaemonJob{
		ctx:     ctx,
		Payload: payload,
	}
}

func NewInstallDaemonTask(serverID, daemonID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.NewTask(TypeInstallDaemon, InstallDaemonPayload{
		ServerID: serverID,
		DaemonID: daemonID,
		UserID:   userID,
	})
}
