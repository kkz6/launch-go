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

const TypeInstallDaemon = "server:install_daemon"

type InstallDaemonPayload struct {
	ServerID string  `json:"server_id"`
	DaemonID string  `json:"daemon_id"`
	UserID   *string `json:"user_id,omitempty"`
}

// InstallDaemonJob installs a daemon (supervisor program) on a server.
type InstallDaemonJob struct {
	Deps    *JobDeps
	Payload InstallDaemonPayload

	daemon *models.Daemon
}

func NewInstallDaemonJob(p InstallDaemonPayload) pkgjobs.Handler {
	return &InstallDaemonJob{Deps: deps, Payload: p}
}

func (j *InstallDaemonJob) Handle(ctx context.Context) error {
	var err error
	j.daemon, err = j.Deps.Repos.Daemon().FindByIDWithServer(ctx, j.Payload.DaemonID)
	if err != nil {
		return fmt.Errorf("failed to find daemon: %w", err)
	}

	contents := j.daemon.ToSupervisorConfig()

	uploadTask := tasks.UploadDaemon(tasks.UploadDaemonConfig{
		Path:         j.daemon.Path(),
		Contents:     contents,
		LogPath:      j.daemon.GetLogPath(),
		ErrorLogPath: j.daemon.GetErrorLogPath(),
		User:         j.daemon.User,
	})

	result, err := j.Deps.RunTask(j.daemon.Server, uploadTask).
		AsRoot().
		Dispatch(ctx)

	if err != nil {
		return fmt.Errorf("failed to upload daemon config: %w", err)
	}

	if !result.IsSuccessful() {
		return fmt.Errorf("failed to upload daemon config: %s", result.GetOutput())
	}

	reloadTask := tasks.ReloadSupervisor()
	_, err = j.Deps.RunTask(j.daemon.Server, reloadTask).
		AsRoot().
		Dispatch(ctx)

	if err != nil {
		j.Deps.Logger.Error().Err(err).Msg("failed to reload supervisor, daemon may not start")
	}

	if err := j.Deps.Repos.Daemon().MarkAsInstalled(ctx, j.daemon.ID); err != nil {
		return fmt.Errorf("failed to mark daemon as installed: %w", err)
	}

	// Log activity
	activity.RecordWithLogPtr(ctx, "server", "installed", j.Payload.UserID, j.daemon, "Daemon was installed")

	j.Deps.Logger.Info().
		Str("daemon_id", j.daemon.ID).
		Str("server_id", j.daemon.ServerID).
		Str("command", j.daemon.Command).
		Msg("daemon installed successfully")

	j.Deps.BroadcastServerEvent(j.daemon.Server, "daemon.installed", map[string]any{
		"daemon_id": j.daemon.ID,
		"server_id": j.daemon.ServerID,
	})

	return nil
}

func (j *InstallDaemonJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).
		Str("daemon_id", j.Payload.DaemonID).
		Str("server_id", j.Payload.ServerID).
		Msg("failed to install daemon")

	// Mark installation as failed
	if markErr := j.Deps.Repos.Daemon().MarkInstallationFailed(ctx, j.Payload.DaemonID); markErr != nil {
		j.Deps.Logger.Error().Err(markErr).Msg("failed to mark daemon installation as failed")
	}
}

// NewInstallDaemonTask creates an asynq task for installing a daemon
// Uses TaskID for deduplication to prevent duplicate daemon installations
func NewInstallDaemonTask(serverID, daemonID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.Task(TypeInstallDaemon, InstallDaemonPayload{
		ServerID: serverID,
		DaemonID: daemonID,
		UserID:   userID,
	}, asynq.TaskID(pkgjobs.Dedup("install_daemon", serverID, daemonID)))
}
