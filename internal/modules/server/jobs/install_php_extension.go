package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeInstallPhpExtension = "server:install_php_extension"

type InstallPhpExtensionPayload struct {
	ServerID  string  `json:"server_id"`
	Version   string  `json:"version"`
	Extension string  `json:"extension"`
	UserID    *string `json:"user_id,omitempty"`
}

// InstallPhpExtensionJob installs a PHP extension on a server
type InstallPhpExtensionJob struct {
	ctx     *JobContext
	Payload InstallPhpExtensionPayload
}

func (j *InstallPhpExtensionJob) Handle(ctx context.Context) error {
	server, err := j.ctx.Repos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	j.ctx.LogInfo("Installing PHP extension",
		"server_id", server.ID,
		"version", j.Payload.Version,
		"extension", j.Payload.Extension,
	)

	task := tasks.InstallPhpExtension(j.Payload.Version, j.Payload.Extension)

	result, err := j.ctx.ForServer(server).RunTask(task).
		AsRoot().
		TrackInDB().
		Dispatch(ctx)

	if err != nil {
		return fmt.Errorf("failed to install PHP extension: %w", err)
	}

	if !result.IsSuccessful() {
		return fmt.Errorf("failed to install PHP extension: %s", result.GetOutput())
	}

	j.ctx.LogInfo("PHP extension installed successfully",
		"server_id", server.ID,
		"version", j.Payload.Version,
		"extension", j.Payload.Extension,
	)

	j.ctx.BroadcastServerEvent(server, "php.extension_installed", map[string]any{
		"server_id": server.ID,
		"version":   j.Payload.Version,
		"extension": j.Payload.Extension,
	})

	return nil
}

func (j *InstallPhpExtensionJob) Failed(ctx context.Context, err error) {
	j.ctx.LogError(err, "Failed to install PHP extension",
		"server_id", j.Payload.ServerID,
		"version", j.Payload.Version,
		"extension", j.Payload.Extension,
	)
}

func NewInstallPhpExtensionJob(ctx *JobContext, payload InstallPhpExtensionPayload) *InstallPhpExtensionJob {
	return &InstallPhpExtensionJob{
		ctx:     ctx,
		Payload: payload,
	}
}

func NewInstallPhpExtensionTask(serverID, version, extension string, userID *string) (*asynq.Task, error) {
	return pkgjobs.NewTask(TypeInstallPhpExtension, InstallPhpExtensionPayload{
		ServerID:  serverID,
		Version:   version,
		Extension: extension,
		UserID:    userID,
	})
}
