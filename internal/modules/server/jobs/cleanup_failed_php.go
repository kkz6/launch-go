package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/notification/notifications"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/modules/server/types"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

// CleanupFailedPhpInstallation job types
const (
	TypeCleanupFailedPhpInstallation       = "server:cleanup_failed_php_installation"
	TypeCleanupFailedPhpExtensionInstall   = "server:cleanup_failed_php_extension_install"
	TypeCleanupFailedPhpExtensionUninstall = "server:cleanup_failed_php_extension_uninstall"
)

// CleanupFailedPhpInstallationPayload holds data for PHP installation cleanup
type CleanupFailedPhpInstallationPayload struct {
	ServerID  string  `json:"server_id"`
	ServiceID string  `json:"service_id"`
	Version   string  `json:"version"`
	UserID    *string `json:"user_id,omitempty"`
}

// CleanupFailedPhpInstallationJob cleans up after a failed PHP installation
type CleanupFailedPhpInstallationJob struct {
	Deps    *JobDeps
	Payload CleanupFailedPhpInstallationPayload

	server *models.Server
}

func NewCleanupFailedPhpInstallationJob(p CleanupFailedPhpInstallationPayload) pkgjobs.Handler {
	return &CleanupFailedPhpInstallationJob{Deps: deps, Payload: p}
}

func (j *CleanupFailedPhpInstallationJob) Handle(ctx context.Context) error {
	var err error

	j.server, err = j.Deps.Repos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("find server: %w", err)
	}

	j.Deps.Logger.Info().
		Str("server_id", j.server.ID).
		Str("version", j.Payload.Version).
		Msg("cleaning up failed PHP installation")

	task := tasks.RemovePhpVersion(j.Payload.Version)
	result, err := j.Deps.RunTask(j.server, task).AsUser().Dispatch(ctx)
	if err != nil {
		j.Deps.Logger.Error().Err(err).
			Str("version", j.Payload.Version).
			Msg("failed to cleanup PHP packages")
	} else if !result.IsSuccessful() {
		j.Deps.Logger.Error().
			Str("version", j.Payload.Version).
			Int("exit_code", result.GetExitCode()).
			Msg("PHP cleanup returned non-zero exit code")
	}

	if j.Payload.ServiceID != "" {
		if err := j.Deps.Repos.Service().UpdateStatus(ctx, j.Payload.ServiceID, types.ServiceStatusFailed); err != nil {
			j.Deps.Logger.Error().Err(err).
				Str("service_id", j.Payload.ServiceID).
				Msg("failed to update service status")
		}
	}

	j.Deps.Logger.Info().
		Str("server_id", j.server.ID).
		Str("version", j.Payload.Version).
		Msg("PHP installation cleanup completed")

	if j.Deps.TaskRunnerDeps.Notifier != nil {
		notif := notifications.NewPhpInstallationFailedNotification(j.server.Name, j.Payload.Version, "", "PHP installation failed")
		_ = j.Deps.TaskRunnerDeps.Notifier.SendToTeam(ctx, j.server.TeamID, notif)
	}

	j.Deps.BroadcastServerEvent(j.server, "php.cleanup_completed", map[string]any{
		"server_id": j.server.ID,
		"version":   j.Payload.Version,
	})

	return nil
}

func (j *CleanupFailedPhpInstallationJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).
		Str("server_id", j.Payload.ServerID).
		Str("version", j.Payload.Version).
		Msg("failed to cleanup failed PHP installation")
}

func NewCleanupFailedPhpInstallationTask(serverID, serviceID, version string, userID *string) (*asynq.Task, error) {
	return pkgjobs.Task(TypeCleanupFailedPhpInstallation, CleanupFailedPhpInstallationPayload{
		ServerID:  serverID,
		ServiceID: serviceID,
		Version:   version,
		UserID:    userID,
	})
}

// CleanupFailedPhpExtensionInstallPayload holds data for extension install cleanup
type CleanupFailedPhpExtensionInstallPayload struct {
	ServerID  string  `json:"server_id"`
	Version   string  `json:"version"`
	Extension string  `json:"extension"`
	UserID    *string `json:"user_id,omitempty"`
}

// CleanupFailedPhpExtensionInstallJob cleans up after a failed extension installation
type CleanupFailedPhpExtensionInstallJob struct {
	Deps    *JobDeps
	Payload CleanupFailedPhpExtensionInstallPayload

	server *models.Server
}

func NewCleanupFailedPhpExtensionInstallJob(p CleanupFailedPhpExtensionInstallPayload) pkgjobs.Handler {
	return &CleanupFailedPhpExtensionInstallJob{Deps: deps, Payload: p}
}

func (j *CleanupFailedPhpExtensionInstallJob) Handle(ctx context.Context) error {
	var err error

	j.server, err = j.Deps.Repos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("find server: %w", err)
	}

	j.Deps.Logger.Info().
		Str("server_id", j.server.ID).
		Str("version", j.Payload.Version).
		Str("extension", j.Payload.Extension).
		Msg("cleaning up failed PHP extension installation")

	task := tasks.UninstallPhpExtension(j.Payload.Version, j.Payload.Extension)
	result, err := j.Deps.RunTask(j.server, task).AsUser().Dispatch(ctx)
	if err != nil {
		j.Deps.Logger.Error().Err(err).
			Str("version", j.Payload.Version).
			Str("extension", j.Payload.Extension).
			Msg("failed to cleanup PHP extension")
	} else if !result.IsSuccessful() {
		j.Deps.Logger.Error().
			Str("version", j.Payload.Version).
			Str("extension", j.Payload.Extension).
			Int("exit_code", result.GetExitCode()).
			Msg("extension cleanup returned non-zero exit code")
	}

	restartTask := tasks.RestartPhp(j.Payload.Version)
	if _, err := j.Deps.RunTask(j.server, restartTask).AsUser().Dispatch(ctx); err != nil {
		j.Deps.Logger.Error().Err(err).
			Msg("failed to restart PHP-FPM after cleanup")
	}

	j.Deps.Logger.Info().
		Str("server_id", j.server.ID).
		Str("version", j.Payload.Version).
		Str("extension", j.Payload.Extension).
		Msg("PHP extension cleanup completed")

	if j.Deps.TaskRunnerDeps.Notifier != nil {
		notif := notifications.NewPhpExtensionInstallFailedNotification(j.server.Name, j.Payload.Extension, j.Payload.Version, "", "PHP extension installation failed")
		_ = j.Deps.TaskRunnerDeps.Notifier.SendToTeam(ctx, j.server.TeamID, notif)
	}

	j.Deps.BroadcastServerEvent(j.server, "php.extension_cleanup_completed", map[string]any{
		"server_id": j.server.ID,
		"version":   j.Payload.Version,
		"extension": j.Payload.Extension,
	})

	return nil
}

func (j *CleanupFailedPhpExtensionInstallJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).
		Str("server_id", j.Payload.ServerID).
		Str("version", j.Payload.Version).
		Str("extension", j.Payload.Extension).
		Msg("failed to cleanup failed PHP extension installation")
}

func NewCleanupFailedPhpExtensionInstallTask(serverID, version, extension string, userID *string) (*asynq.Task, error) {
	return pkgjobs.Task(TypeCleanupFailedPhpExtensionInstall, CleanupFailedPhpExtensionInstallPayload{
		ServerID:  serverID,
		Version:   version,
		Extension: extension,
		UserID:    userID,
	})
}

// CleanupFailedPhpExtensionUninstallPayload holds data for extension uninstall cleanup
type CleanupFailedPhpExtensionUninstallPayload struct {
	ServerID  string  `json:"server_id"`
	Version   string  `json:"version"`
	Extension string  `json:"extension"`
	UserID    *string `json:"user_id,omitempty"`
}

// CleanupFailedPhpExtensionUninstallJob cleans up after a failed extension uninstallation
type CleanupFailedPhpExtensionUninstallJob struct {
	Deps    *JobDeps
	Payload CleanupFailedPhpExtensionUninstallPayload

	server *models.Server
}

func NewCleanupFailedPhpExtensionUninstallJob(p CleanupFailedPhpExtensionUninstallPayload) pkgjobs.Handler {
	return &CleanupFailedPhpExtensionUninstallJob{Deps: deps, Payload: p}
}

func (j *CleanupFailedPhpExtensionUninstallJob) Handle(ctx context.Context) error {
	var err error

	j.server, err = j.Deps.Repos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("find server: %w", err)
	}

	j.Deps.Logger.Info().
		Str("server_id", j.server.ID).
		Str("version", j.Payload.Version).
		Str("extension", j.Payload.Extension).
		Msg("cleaning up failed PHP extension uninstallation")

	restartTask := tasks.RestartPhp(j.Payload.Version)
	if _, err := j.Deps.RunTask(j.server, restartTask).AsUser().Dispatch(ctx); err != nil {
		j.Deps.Logger.Error().Err(err).
			Msg("failed to restart PHP-FPM after cleanup")
	}

	j.Deps.Logger.Info().
		Str("server_id", j.server.ID).
		Str("version", j.Payload.Version).
		Str("extension", j.Payload.Extension).
		Msg("PHP extension uninstall cleanup completed")

	if j.Deps.TaskRunnerDeps.Notifier != nil {
		notif := notifications.NewPhpExtensionUninstallFailedNotification(j.server.Name, j.Payload.Extension, j.Payload.Version, "", "PHP extension uninstallation failed")
		_ = j.Deps.TaskRunnerDeps.Notifier.SendToTeam(ctx, j.server.TeamID, notif)
	}

	j.Deps.BroadcastServerEvent(j.server, "php.extension_uninstall_cleanup_completed", map[string]any{
		"server_id": j.server.ID,
		"version":   j.Payload.Version,
		"extension": j.Payload.Extension,
	})

	return nil
}

func (j *CleanupFailedPhpExtensionUninstallJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).
		Str("server_id", j.Payload.ServerID).
		Str("version", j.Payload.Version).
		Str("extension", j.Payload.Extension).
		Msg("failed to cleanup failed PHP extension uninstallation")
}

func NewCleanupFailedPhpExtensionUninstallTask(serverID, version, extension string, userID *string) (*asynq.Task, error) {
	return pkgjobs.Task(TypeCleanupFailedPhpExtensionUninstall, CleanupFailedPhpExtensionUninstallPayload{
		ServerID:  serverID,
		Version:   version,
		Extension: extension,
		UserID:    userID,
	})
}
