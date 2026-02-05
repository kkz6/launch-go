package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/models"
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
	Deps    *JobDeps
	Payload InstallPhpExtensionPayload

	server *models.Server
}

func NewInstallPhpExtensionJob(p InstallPhpExtensionPayload) pkgjobs.Handler {
	return &InstallPhpExtensionJob{Deps: deps, Payload: p}
}

func (j *InstallPhpExtensionJob) Handle(ctx context.Context) error {
	var err error
	j.server, err = j.Deps.Repos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	j.Deps.Logger.Info().
		Str("server_id", j.server.ID).
		Str("version", j.Payload.Version).
		Str("extension", j.Payload.Extension).
		Msg("installing PHP extension")

	task := tasks.InstallPhpExtension(j.Payload.Version, j.Payload.Extension)

	result, err := j.Deps.RunTask(j.server, task).
		AsRoot().
		TrackInDB().
		Dispatch(ctx)

	if err != nil {
		return fmt.Errorf("failed to install PHP extension: %w", err)
	}

	if !result.IsSuccessful() {
		return fmt.Errorf("failed to install PHP extension: %s", result.GetOutput())
	}

	// Find the PHP service and update its extensions
	phpService, err := j.Deps.Repos.Service().FindPhpByServerAndVersion(ctx, j.server.ID, j.Payload.Version)
	if err != nil {
		j.Deps.Logger.Warn().Err(err).
			Str("server_id", j.server.ID).
			Str("version", j.Payload.Version).
			Msg("failed to find PHP service to update extensions")
	} else {
		if err := j.Deps.Repos.Service().AddExtension(ctx, phpService.ID, j.Payload.Extension); err != nil {
			j.Deps.Logger.Warn().Err(err).
				Str("server_id", j.server.ID).
				Str("version", j.Payload.Version).
				Str("extension", j.Payload.Extension).
				Msg("failed to update extension in database")
		}
	}

	j.Deps.Logger.Info().
		Str("server_id", j.server.ID).
		Str("version", j.Payload.Version).
		Str("extension", j.Payload.Extension).
		Msg("PHP extension installed successfully")

	j.Deps.BroadcastServerEvent(j.server, "php.extension_installed", map[string]any{
		"server_id": j.server.ID,
		"version":   j.Payload.Version,
		"extension": j.Payload.Extension,
	})

	return nil
}

func (j *InstallPhpExtensionJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).
		Str("server_id", j.Payload.ServerID).
		Str("version", j.Payload.Version).
		Str("extension", j.Payload.Extension).
		Msg("failed to install PHP extension")
}

func NewInstallPhpExtensionTask(serverID, version, extension string, userID *string) (*asynq.Task, error) {
	return pkgjobs.Task(TypeInstallPhpExtension, InstallPhpExtensionPayload{
		ServerID:  serverID,
		Version:   version,
		Extension: extension,
		UserID:    userID,
	}, asynq.TaskID(pkgjobs.Dedup("install_php_ext", serverID, version, extension)))
}
