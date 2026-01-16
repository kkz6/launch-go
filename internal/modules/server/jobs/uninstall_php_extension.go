package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeUninstallPhpExtension = "server:uninstall_php_extension"

type UninstallPhpExtensionPayload struct {
	ServerID  string  `json:"server_id"`
	Version   string  `json:"version"`
	Extension string  `json:"extension"`
	UserID    *string `json:"user_id,omitempty"`
}

// UninstallPhpExtensionJob uninstalls a PHP extension from a server
type UninstallPhpExtensionJob struct {
	ctx     *JobContext
	Payload UninstallPhpExtensionPayload
}

func (j *UninstallPhpExtensionJob) Handle(ctx context.Context) error {
	server, err := j.ctx.Repos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	j.ctx.LogInfo("Uninstalling PHP extension",
		"server_id", server.ID,
		"version", j.Payload.Version,
		"extension", j.Payload.Extension,
	)

	task := tasks.UninstallPhpExtension(j.Payload.Version, j.Payload.Extension)

	result, err := j.ctx.ForServer(server).RunTask(task).
		AsRoot().
		TrackInDB().
		Dispatch(ctx)

	if err != nil {
		return fmt.Errorf("failed to uninstall PHP extension: %w", err)
	}

	if !result.IsSuccessful() {
		return fmt.Errorf("failed to uninstall PHP extension: %s", result.GetOutput())
	}

	j.ctx.LogInfo("PHP extension uninstalled successfully",
		"server_id", server.ID,
		"version", j.Payload.Version,
		"extension", j.Payload.Extension,
	)

	j.ctx.BroadcastServerEvent(server, "php.extension_uninstalled", map[string]any{
		"server_id": server.ID,
		"version":   j.Payload.Version,
		"extension": j.Payload.Extension,
	})

	return nil
}

func (j *UninstallPhpExtensionJob) Failed(ctx context.Context, err error) {
	j.ctx.LogError(err, "Failed to uninstall PHP extension",
		"server_id", j.Payload.ServerID,
		"version", j.Payload.Version,
		"extension", j.Payload.Extension,
	)
}

func NewUninstallPhpExtensionJob(ctx *JobContext, payload UninstallPhpExtensionPayload) *UninstallPhpExtensionJob {
	return &UninstallPhpExtensionJob{
		ctx:     ctx,
		Payload: payload,
	}
}

func NewUninstallPhpExtensionTask(serverID, version, extension string, userID *string) (*asynq.Task, error) {
	return pkgjobs.NewTask(TypeUninstallPhpExtension, UninstallPhpExtensionPayload{
		ServerID:  serverID,
		Version:   version,
		Extension: extension,
		UserID:    userID,
	})
}
