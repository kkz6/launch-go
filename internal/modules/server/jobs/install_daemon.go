package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/pkg/jobs"
)

// InstallDaemonJob installs a daemon (supervisor program) on a server.
type InstallDaemonJob struct {
	ServerJobBase
	jobs.InstallationTracker
	Payload InstallDaemonPayload
}

func (j *InstallDaemonJob) Type() string {
	return TypeInstallDaemon
}

func (j *InstallDaemonJob) Handle(ctx context.Context) error {
	daemon, err := j.Repo().FindDaemonByIDWithServer(ctx, j.Payload.DaemonID)
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

	result, err := j.RunTaskOnServer(daemon.Server, uploadTask).
		AsRoot().
		Dispatch(ctx)

	if err != nil {
		return fmt.Errorf("failed to upload daemon config: %w", err)
	}

	if !result.IsSuccessful() {
		return fmt.Errorf("failed to upload daemon config: %s", result.GetOutput())
	}

	reloadTask := tasks.ReloadSupervisor()
	_, err = j.RunTaskOnServer(daemon.Server, reloadTask).
		AsRoot().
		Dispatch(ctx)

	if err != nil {
		j.LogError(err, "Failed to reload supervisor, daemon may not start")
	}

	if err := j.Repo().MarkDaemonInstalled(ctx, daemon.ID); err != nil {
		return fmt.Errorf("failed to mark daemon as installed: %w", err)
	}

	j.LogInfo("Daemon installed successfully",
		"daemon_id", daemon.ID,
		"server_id", daemon.ServerID,
		"command", daemon.Command,
	)

	j.BroadcastServerEvent(daemon.ServerID, "daemon.installed", map[string]any{
		"daemon_id": daemon.ID,
		"server_id": daemon.ServerID,
	})

	return nil
}

func (j *InstallDaemonJob) Failed(ctx context.Context, err error) {
	j.LogError(err, "Failed to install daemon",
		"daemon_id", j.Payload.DaemonID,
		"server_id", j.Payload.ServerID,
	)

	daemon, findErr := j.Repo().FindDaemonByID(ctx, j.Payload.DaemonID)
	if findErr == nil && daemon != nil {
		j.MarkInstallationFailed(j.DB, daemon)
	}
}
func NewInstallDaemonTask(serverID, daemonID string, userID *string) (*asynq.Task, error) {
	return jobs.NewTask(TypeInstallDaemon, InstallDaemonPayload{
		ServerID: serverID,
		DaemonID: daemonID,
		UserID:   userID,
	})
}
