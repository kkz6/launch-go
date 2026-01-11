package tasks

import (
	"context"
	"time"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/taskrunner"
)

// RemovePhpVersion removes a PHP version from a server
type RemovePhpVersion struct {
	BaseServerTask
	service *models.InstalledService
	version string
}

// NewRemovePhpVersion creates a new RemovePhpVersion task
func NewRemovePhpVersion(server *models.Server, service *models.InstalledService, version string) *RemovePhpVersion {
	task := &RemovePhpVersion{
		BaseServerTask: BaseServerTask{
			BaseTask: taskrunner.BaseTask{
				TemplateName: "server/php/remove-version",
				TaskTimeout:  5 * time.Minute,
			},
			server: server,
		},
		service: service,
		version: version,
	}

	task.FinishedCallback = task.onFinished
	task.FailedCallback = task.onFailed
	task.TimeoutCallback = task.onTimeout

	return task
}

// Data returns the template data
func (t *RemovePhpVersion) Data() map[string]interface{} {
	return map[string]interface{}{
		"Server":     t.server,
		"Service":    t.service,
		"Version":    t.version,
		"PhpVersion": t.version,
	}
}

// Service returns the service being removed
func (t *RemovePhpVersion) Service() *models.InstalledService {
	return t.service
}

// Version returns the PHP version being removed
func (t *RemovePhpVersion) Version() string {
	return t.version
}

// onFinished handles successful completion
func (t *RemovePhpVersion) onFinished(ctx context.Context, result *taskrunner.TaskResult) {
	// Delete the service record
	// This would be handled by the job/repository layer
}

// onFailed handles task failure
func (t *RemovePhpVersion) onFailed(ctx context.Context, result *taskrunner.TaskResult) {
	// Update service status to failed
	// Log the error
}

// onTimeout handles task timeout
func (t *RemovePhpVersion) onTimeout(ctx context.Context, result *taskrunner.TaskResult) {
	// Update service status to failed
	// Log the timeout
}
