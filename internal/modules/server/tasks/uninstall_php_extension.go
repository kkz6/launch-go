package tasks

import (
	"context"
	"time"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// UninstallPhpExtension uninstalls a PHP extension from a server
type UninstallPhpExtension struct {
	BaseServerTask
	service     *models.InstalledService
	extension   string
	hasCallback bool
}

// NewUninstallPhpExtension creates a new UninstallPhpExtension task
func NewUninstallPhpExtension(server *models.Server, service *models.InstalledService, extension string) *UninstallPhpExtension {
	task := &UninstallPhpExtension{
		BaseServerTask: BaseServerTask{
			BaseTask: taskrunner.BaseTask{
				TemplateName: "server/php/extensions/uninstall",
				TaskTimeout:  2 * time.Minute,
			},
			server: server,
		},
		service:     service,
		extension:   extension,
		hasCallback: true,
	}

	task.FinishedCallback = task.onFinished
	task.FailedCallback = task.onFailed
	task.TimeoutCallback = task.onTimeout

	return task
}

// NewUninstallPhpExtensionWithoutCallback creates a new UninstallPhpExtension task without callbacks
func NewUninstallPhpExtensionWithoutCallback(server *models.Server, service *models.InstalledService, extension string) *UninstallPhpExtension {
	task := NewUninstallPhpExtension(server, service, extension)
	task.hasCallback = false

	return task
}

// Data returns the template data
func (t *UninstallPhpExtension) Data() map[string]interface{} {
	return map[string]interface{}{
		"Server":     t.server,
		"Service":    t.service,
		"Extension":  t.extension,
		"PhpVersion": t.service.Version,
	}
}

// Service returns the PHP service
func (t *UninstallPhpExtension) Service() *models.InstalledService {
	return t.service
}

// Extension returns the extension being uninstalled
func (t *UninstallPhpExtension) Extension() string {
	return t.extension
}

// HasCallback returns whether the task has callbacks
func (t *UninstallPhpExtension) HasCallback() bool {
	return t.hasCallback
}

// onFinished handles successful completion
func (t *UninstallPhpExtension) onFinished(ctx context.Context, result *taskrunner.TaskResult) {
	// Remove extension from service type_data
	// delete(typeData["extensions"], extension)
}

// onFailed handles task failure
func (t *UninstallPhpExtension) onFailed(ctx context.Context, result *taskrunner.TaskResult) {
	// Update extension status to failed in service type_data
	// Dispatch CleanupFailedPhpExtensionUninstall job
}

// onTimeout handles task timeout
func (t *UninstallPhpExtension) onTimeout(ctx context.Context, result *taskrunner.TaskResult) {
	// Update extension status to failed in service type_data
	// Dispatch CleanupFailedPhpExtensionUninstall job with timeout message
}
