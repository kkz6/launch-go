package tasks

import (
	"context"
	"time"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/taskrunner"
)

// InstallPhpExtension installs a PHP extension on a server
type InstallPhpExtension struct {
	BaseServerTask
	service     *models.InstalledService
	extension   string
	hasCallback bool
}

// NewInstallPhpExtension creates a new InstallPhpExtension task
func NewInstallPhpExtension(server *models.Server, service *models.InstalledService, extension string) *InstallPhpExtension {
	task := &InstallPhpExtension{
		BaseServerTask: BaseServerTask{
			BaseTask: taskrunner.BaseTask{
				TemplateName: "server/php/extensions/install",
				TaskTimeout:  3 * time.Minute,
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

// NewInstallPhpExtensionWithoutCallback creates a new InstallPhpExtension task without callbacks
func NewInstallPhpExtensionWithoutCallback(server *models.Server, service *models.InstalledService, extension string) *InstallPhpExtension {
	task := NewInstallPhpExtension(server, service, extension)
	task.hasCallback = false

	return task
}

// Data returns the template data
func (t *InstallPhpExtension) Data() map[string]interface{} {
	return map[string]interface{}{
		"Server":     t.server,
		"Service":    t.service,
		"Extension":  t.extension,
		"PhpVersion": t.service.Version,
	}
}

// Service returns the PHP service
func (t *InstallPhpExtension) Service() *models.InstalledService {
	return t.service
}

// Extension returns the extension being installed
func (t *InstallPhpExtension) Extension() string {
	return t.extension
}

// HasCallback returns whether the task has callbacks
func (t *InstallPhpExtension) HasCallback() bool {
	return t.hasCallback
}

// onFinished handles successful completion
func (t *InstallPhpExtension) onFinished(ctx context.Context, result *taskrunner.TaskResult) {
	// Update extension status to installed in service type_data
	// typeData["extensions"][extension] = "installed"
}

// onFailed handles task failure
func (t *InstallPhpExtension) onFailed(ctx context.Context, result *taskrunner.TaskResult) {
	// Update extension status to failed in service type_data
	// Dispatch CleanupFailedPhpExtensionInstall job
}

// onTimeout handles task timeout
func (t *InstallPhpExtension) onTimeout(ctx context.Context, result *taskrunner.TaskResult) {
	// Update extension status to failed in service type_data
	// Dispatch CleanupFailedPhpExtensionInstall job with timeout message
}
