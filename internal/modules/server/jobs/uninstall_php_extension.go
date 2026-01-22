package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/models"
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
	Deps    *JobDeps
	Payload UninstallPhpExtensionPayload

	server *models.Server
}

func NewUninstallPhpExtensionJob(p UninstallPhpExtensionPayload) pkgjobs.Handler {
	return &UninstallPhpExtensionJob{Deps: deps, Payload: p}
}

func (j *UninstallPhpExtensionJob) Handle(ctx context.Context) error {
	var err error
	j.server, err = j.Deps.Repos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	j.Deps.Logger.Info().
		Str("server_id", j.server.ID).
		Str("version", j.Payload.Version).
		Str("extension", j.Payload.Extension).
		Msg("Uninstalling PHP extension")

	task := tasks.UninstallPhpExtension(j.Payload.Version, j.Payload.Extension)

	result, err := j.Deps.RunTask(j.server, task).
		AsRoot().
		TrackInDB().
		Dispatch(ctx)

	if err != nil {
		return fmt.Errorf("failed to uninstall PHP extension: %w", err)
	}

	if !result.IsSuccessful() {
		return fmt.Errorf("failed to uninstall PHP extension: %s", result.GetOutput())
	}

	j.Deps.Logger.Info().
		Str("server_id", j.server.ID).
		Str("version", j.Payload.Version).
		Str("extension", j.Payload.Extension).
		Msg("PHP extension uninstalled successfully")

	j.Deps.BroadcastServerEvent(j.server, "php.extension_uninstalled", map[string]any{
		"server_id": j.server.ID,
		"version":   j.Payload.Version,
		"extension": j.Payload.Extension,
	})

	return nil
}

func (j *UninstallPhpExtensionJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).
		Str("server_id", j.Payload.ServerID).
		Str("version", j.Payload.Version).
		Str("extension", j.Payload.Extension).
		Msg("Failed to uninstall PHP extension")
}

func NewUninstallPhpExtensionTask(serverID, version, extension string, userID *string) (*asynq.Task, error) {
	return pkgjobs.Task(TypeUninstallPhpExtension, UninstallPhpExtensionPayload{
		ServerID:  serverID,
		Version:   version,
		Extension: extension,
		UserID:    userID,
	})
}
