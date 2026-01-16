package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/enums"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
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
	ctx     *JobContext
	Payload CleanupFailedPhpInstallationPayload
}

// NewCleanupFailedPhpInstallationJob creates a new cleanup job
func NewCleanupFailedPhpInstallationJob(ctx *JobContext, payload CleanupFailedPhpInstallationPayload) *CleanupFailedPhpInstallationJob {
	return &CleanupFailedPhpInstallationJob{
		ctx:     ctx,
		Payload: payload,
	}
}

// Handle executes the cleanup job
func (j *CleanupFailedPhpInstallationJob) Handle(ctx context.Context) error {
	server, err := j.ctx.Repos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	j.ctx.LogInfo("Cleaning up failed PHP installation",
		"server_id", server.ID,
		"version", j.Payload.Version,
	)

	// Try to remove any partially installed PHP packages
	task := tasks.RemovePhpVersion(j.Payload.Version)
	result, err := j.ctx.ForServer(server).RunTask(task).AsRoot().Dispatch(ctx)
	if err != nil {
		j.ctx.LogError(err, "Failed to cleanup PHP packages", "version", j.Payload.Version)
		// Continue with service cleanup even if removal fails
	} else if !result.IsSuccessful() {
		j.ctx.LogError(nil, "PHP cleanup returned non-zero exit code",
			"version", j.Payload.Version,
			"exit_code", result.GetExitCode(),
		)
	}

	// Update service status to failed if service exists
	if j.Payload.ServiceID != "" {
		if err := j.ctx.Repos.Service().UpdateStatus(ctx, j.Payload.ServiceID, enums.ServiceStatusFailed); err != nil {
			j.ctx.LogError(err, "Failed to update service status", "service_id", j.Payload.ServiceID)
		}
	}

	j.ctx.LogInfo("PHP installation cleanup completed",
		"server_id", server.ID,
		"version", j.Payload.Version,
	)

	j.ctx.BroadcastServerEvent(server, "php.cleanup_completed", map[string]any{
		"server_id": server.ID,
		"version":   j.Payload.Version,
	})

	return nil
}

// Failed handles job failure
func (j *CleanupFailedPhpInstallationJob) Failed(ctx context.Context, err error) {
	j.ctx.LogError(err, "Failed to cleanup failed PHP installation",
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
	ctx     *JobContext
	Payload CleanupFailedPhpExtensionInstallPayload
}

// NewCleanupFailedPhpExtensionInstallJob creates a new cleanup job
func NewCleanupFailedPhpExtensionInstallJob(ctx *JobContext, payload CleanupFailedPhpExtensionInstallPayload) *CleanupFailedPhpExtensionInstallJob {
	return &CleanupFailedPhpExtensionInstallJob{
		ctx:     ctx,
		Payload: payload,
	}
}

// Handle executes the cleanup job
func (j *CleanupFailedPhpExtensionInstallJob) Handle(ctx context.Context) error {
	server, err := j.ctx.Repos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	j.ctx.LogInfo("Cleaning up failed PHP extension installation",
		"server_id", server.ID,
		"version", j.Payload.Version,
		"extension", j.Payload.Extension,
	)

	// Try to remove the partially installed extension
	task := tasks.UninstallPhpExtension(j.Payload.Version, j.Payload.Extension)
	result, err := j.ctx.ForServer(server).RunTask(task).AsRoot().Dispatch(ctx)
	if err != nil {
		j.ctx.LogError(err, "Failed to cleanup PHP extension",
			"version", j.Payload.Version,
			"extension", j.Payload.Extension,
		)
	} else if !result.IsSuccessful() {
		j.ctx.LogError(nil, "Extension cleanup returned non-zero exit code",
			"version", j.Payload.Version,
			"extension", j.Payload.Extension,
			"exit_code", result.GetExitCode(),
		)
	}

	// Restart PHP-FPM to ensure clean state
	restartTask := tasks.RestartPhp(j.Payload.Version)
	if _, err := j.ctx.ForServer(server).RunTask(restartTask).AsRoot().Dispatch(ctx); err != nil {
		j.ctx.LogError(err, "Failed to restart PHP-FPM after cleanup")
	}

	j.ctx.LogInfo("PHP extension cleanup completed",
		"server_id", server.ID,
		"version", j.Payload.Version,
		"extension", j.Payload.Extension,
	)

	j.ctx.BroadcastServerEvent(server, "php.extension_cleanup_completed", map[string]any{
		"server_id": server.ID,
		"version":   j.Payload.Version,
		"extension": j.Payload.Extension,
	})

	return nil
}

// Failed handles job failure
func (j *CleanupFailedPhpExtensionInstallJob) Failed(ctx context.Context, err error) {
	j.ctx.LogError(err, "Failed to cleanup failed PHP extension installation",
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
	ctx     *JobContext
	Payload CleanupFailedPhpExtensionUninstallPayload
}

// NewCleanupFailedPhpExtensionUninstallJob creates a new cleanup job
func NewCleanupFailedPhpExtensionUninstallJob(ctx *JobContext, payload CleanupFailedPhpExtensionUninstallPayload) *CleanupFailedPhpExtensionUninstallJob {
	return &CleanupFailedPhpExtensionUninstallJob{
		ctx:     ctx,
		Payload: payload,
	}
}

// Handle executes the cleanup job
func (j *CleanupFailedPhpExtensionUninstallJob) Handle(ctx context.Context) error {
	server, err := j.ctx.Repos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	j.ctx.LogInfo("Cleaning up failed PHP extension uninstallation",
		"server_id", server.ID,
		"version", j.Payload.Version,
		"extension", j.Payload.Extension,
	)

	// Restart PHP-FPM to ensure clean state - the extension might still be loaded
	restartTask := tasks.RestartPhp(j.Payload.Version)
	if _, err := j.ctx.ForServer(server).RunTask(restartTask).AsRoot().Dispatch(ctx); err != nil {
		j.ctx.LogError(err, "Failed to restart PHP-FPM after cleanup")
	}

	j.ctx.LogInfo("PHP extension uninstall cleanup completed",
		"server_id", server.ID,
		"version", j.Payload.Version,
		"extension", j.Payload.Extension,
	)

	j.ctx.BroadcastServerEvent(server, "php.extension_uninstall_cleanup_completed", map[string]any{
		"server_id": server.ID,
		"version":   j.Payload.Version,
		"extension": j.Payload.Extension,
	})

	return nil
}

// Failed handles job failure
func (j *CleanupFailedPhpExtensionUninstallJob) Failed(ctx context.Context, err error) {
	j.ctx.LogError(err, "Failed to cleanup failed PHP extension uninstallation",
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
