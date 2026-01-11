package tasks

import (
	"context"
	"fmt"
	"time"

	"github.com/kkz6/launch-go/internal/modules/server/enums"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/taskrunner"
)

// AddPhpVersion installs a PHP version on a server
type AddPhpVersion struct {
	BaseServerTask
	phpVersion enums.Software
}

// NewAddPhpVersion creates a new AddPhpVersion task
func NewAddPhpVersion(server *models.Server, phpVersion enums.Software) *AddPhpVersion {
	task := &AddPhpVersion{
		BaseServerTask: BaseServerTask{
			BaseTask: taskrunner.BaseTask{
				TemplateName: fmt.Sprintf("server/software/install-%s", phpVersion),
				TaskTimeout:  10 * time.Minute,
			},
			server: server,
		},
		phpVersion: phpVersion,
	}

	// Set default callbacks
	task.FinishedCallback = task.onFinished
	task.FailedCallback = task.onFailed
	task.TimeoutCallback = task.onTimeout

	return task
}

// Data returns the template data
func (t *AddPhpVersion) Data() map[string]interface{} {
	return map[string]interface{}{
		"Server":             t.server,
		"Username":           t.server.GetUsername(),
		"MaxChildrenPhpPool": taskrunner.MaxChildrenPhpPool(t.GetMemoryMB()),
		"PhpVersion":         t.phpVersion.GetVersion(),
	}
}

// PhpVersion returns the PHP version being installed
func (t *AddPhpVersion) PhpVersion() enums.Software {
	return t.phpVersion
}

// MaxChildrenPhpPoolValue returns the calculated max children for PHP-FPM pool
func (t *AddPhpVersion) MaxChildrenPhpPoolValue() int {
	return taskrunner.MaxChildrenPhpPool(t.GetMemoryMB())
}

// onFinished handles successful completion
func (t *AddPhpVersion) onFinished(ctx context.Context, result *taskrunner.TaskResult) {
	// Update service status to installed
	// This would be handled by the job/repository layer
}

// onFailed handles task failure
func (t *AddPhpVersion) onFailed(ctx context.Context, result *taskrunner.TaskResult) {
	// Update service status to failed
	// Dispatch cleanup job
	// Fire PhpInstallFailed event
}

// onTimeout handles task timeout
func (t *AddPhpVersion) onTimeout(ctx context.Context, result *taskrunner.TaskResult) {
	// Update service status to failed
	// Dispatch cleanup job with timeout message
	// Delete the service
}
