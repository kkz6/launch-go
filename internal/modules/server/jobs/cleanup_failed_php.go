package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

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
	pkgjobs.BaseJob[*JobContext, CleanupFailedPhpInstallationPayload]
}

// NewCleanupFailedPhpInstallationJob creates a new cleanup job
func NewCleanupFailedPhpInstallationJob(ctx *JobContext, payload CleanupFailedPhpInstallationPayload) *CleanupFailedPhpInstallationJob {
	return &CleanupFailedPhpInstallationJob{
		BaseJob: pkgjobs.NewBaseJob(ctx, payload),
	}
}

// Handle executes the cleanup job
func (j *CleanupFailedPhpInstallationJob) Handle(ctx context.Context) error {
	server, err := j.Ctx.Repos().Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	j.Ctx.LogInfo("Cleaning up failed PHP installation",
		"server_id", server.ID,
		"version", j.Payload.Version,
	)

	// Try to remove any partially installed PHP packages
	task := tasks.RemovePhpVersion(j.Payload.Version)
	result, err := j.Ctx.ForServer(server).RunTask(task).AsRoot().Dispatch(ctx)
	if err != nil {
		j.Ctx.LogError(err, "Failed to cleanup PHP packages", "version", j.Payload.Version)
		// Continue with service cleanup even if removal fails
	} else if !result.IsSuccessful() {
		j.Ctx.LogError(nil, "PHP cleanup returned non-zero exit code",
			"version", j.Payload.Version,
			"exit_code", result.GetExitCode(),
		)
	}

	// Update service status to failed if service exists
	if j.Payload.ServiceID != "" {
		if err := j.Ctx.Repos().Service().UpdateStatus(ctx, j.Payload.ServiceID, types.ServiceStatusFailed); err != nil {
			j.Ctx.LogError(err, "Failed to update service status", "service_id", j.Payload.ServiceID)
		}
	}

	j.Ctx.LogInfo("PHP installation cleanup completed",
		"server_id", server.ID,
		"version", j.Payload.Version,
	)

	j.Ctx.BroadcastServerEvent(server, "php.cleanup_completed", map[string]any{
		"server_id": server.ID,
		"version":   j.Payload.Version,
	})

	return nil
}

// Failed handles job failure
func (j *CleanupFailedPhpInstallationJob) Failed(ctx context.Context, err error) {
	j.Ctx.LogError(err, "Failed to cleanup failed PHP installation",
		"server_id", j.Payload.ServerID,
		"version", j.Payload.Version,
	)
}

// NewCleanupFailedPhpInstallationTask creates a cleanup task
func NewCleanupFailedPhpInstallationTask(serverID, serviceID, version string, userID *string) (*asynq.Task, error) {
	return pkgjobs.NewTask(TypeCleanupFailedPhpInstallation, CleanupFailedPhpInstallationPayload{
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
	pkgjobs.BaseJob[*JobContext, CleanupFailedPhpExtensionInstallPayload]
}

// NewCleanupFailedPhpExtensionInstallJob creates a new cleanup job
func NewCleanupFailedPhpExtensionInstallJob(ctx *JobContext, payload CleanupFailedPhpExtensionInstallPayload) *CleanupFailedPhpExtensionInstallJob {
	return &CleanupFailedPhpExtensionInstallJob{
		BaseJob: pkgjobs.NewBaseJob(ctx, payload),
	}
}

// Handle executes the cleanup job
func (j *CleanupFailedPhpExtensionInstallJob) Handle(ctx context.Context) error {
	server, err := j.Ctx.Repos().Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	j.Ctx.LogInfo("Cleaning up failed PHP extension installation",
		"server_id", server.ID,
		"version", j.Payload.Version,
		"extension", j.Payload.Extension,
	)

	// Try to remove the partially installed extension
	task := tasks.UninstallPhpExtension(j.Payload.Version, j.Payload.Extension)
	result, err := j.Ctx.ForServer(server).RunTask(task).AsRoot().Dispatch(ctx)
	if err != nil {
		j.Ctx.LogError(err, "Failed to cleanup PHP extension",
			"version", j.Payload.Version,
			"extension", j.Payload.Extension,
		)
	} else if !result.IsSuccessful() {
		j.Ctx.LogError(nil, "Extension cleanup returned non-zero exit code",
			"version", j.Payload.Version,
			"extension", j.Payload.Extension,
			"exit_code", result.GetExitCode(),
		)
	}

	// Restart PHP-FPM to ensure clean state
	restartTask := tasks.RestartPhp(j.Payload.Version)
	if _, err := j.Ctx.ForServer(server).RunTask(restartTask).AsRoot().Dispatch(ctx); err != nil {
		j.Ctx.LogError(err, "Failed to restart PHP-FPM after cleanup")
	}

	j.Ctx.LogInfo("PHP extension cleanup completed",
		"server_id", server.ID,
		"version", j.Payload.Version,
		"extension", j.Payload.Extension,
	)

	j.Ctx.BroadcastServerEvent(server, "php.extension_cleanup_completed", map[string]any{
		"server_id": server.ID,
		"version":   j.Payload.Version,
		"extension": j.Payload.Extension,
	})

	return nil
}

// Failed handles job failure
func (j *CleanupFailedPhpExtensionInstallJob) Failed(ctx context.Context, err error) {
	j.Ctx.LogError(err, "Failed to cleanup failed PHP extension installation",
		"server_id", j.Payload.ServerID,
		"version", j.Payload.Version,
		"extension", j.Payload.Extension,
	)
}

// NewCleanupFailedPhpExtensionInstallTask creates a cleanup task
func NewCleanupFailedPhpExtensionInstallTask(serverID, version, extension string, userID *string) (*asynq.Task, error) {
	return pkgjobs.NewTask(TypeCleanupFailedPhpExtensionInstall, CleanupFailedPhpExtensionInstallPayload{
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
	pkgjobs.BaseJob[*JobContext, CleanupFailedPhpExtensionUninstallPayload]
}

// NewCleanupFailedPhpExtensionUninstallJob creates a new cleanup job
func NewCleanupFailedPhpExtensionUninstallJob(ctx *JobContext, payload CleanupFailedPhpExtensionUninstallPayload) *CleanupFailedPhpExtensionUninstallJob {
	return &CleanupFailedPhpExtensionUninstallJob{
		BaseJob: pkgjobs.NewBaseJob(ctx, payload),
	}
}

// Handle executes the cleanup job
func (j *CleanupFailedPhpExtensionUninstallJob) Handle(ctx context.Context) error {
	server, err := j.Ctx.Repos().Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	j.Ctx.LogInfo("Cleaning up failed PHP extension uninstallation",
		"server_id", server.ID,
		"version", j.Payload.Version,
		"extension", j.Payload.Extension,
	)

	// Restart PHP-FPM to ensure clean state - the extension might still be loaded
	restartTask := tasks.RestartPhp(j.Payload.Version)
	if _, err := j.Ctx.ForServer(server).RunTask(restartTask).AsRoot().Dispatch(ctx); err != nil {
		j.Ctx.LogError(err, "Failed to restart PHP-FPM after cleanup")
	}

	j.Ctx.LogInfo("PHP extension uninstall cleanup completed",
		"server_id", server.ID,
		"version", j.Payload.Version,
		"extension", j.Payload.Extension,
	)

	j.Ctx.BroadcastServerEvent(server, "php.extension_uninstall_cleanup_completed", map[string]any{
		"server_id": server.ID,
		"version":   j.Payload.Version,
		"extension": j.Payload.Extension,
	})

	return nil
}

// Failed handles job failure
func (j *CleanupFailedPhpExtensionUninstallJob) Failed(ctx context.Context, err error) {
	j.Ctx.LogError(err, "Failed to cleanup failed PHP extension uninstallation",
		"server_id", j.Payload.ServerID,
		"version", j.Payload.Version,
		"extension", j.Payload.Extension,
	)
}

// NewCleanupFailedPhpExtensionUninstallTask creates a cleanup task
func NewCleanupFailedPhpExtensionUninstallTask(serverID, version, extension string, userID *string) (*asynq.Task, error) {
	return pkgjobs.NewTask(TypeCleanupFailedPhpExtensionUninstall, CleanupFailedPhpExtensionUninstallPayload{
		ServerID:  serverID,
		Version:   version,
		Extension: extension,
		UserID:    userID,
	})
}
