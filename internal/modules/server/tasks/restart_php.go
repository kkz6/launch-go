package tasks

import (
	"context"
	"fmt"
	"time"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/taskrunner"
)

// RestartPhp restarts the PHP-FPM service for a specific version
type RestartPhp struct {
	BaseServerTask
	version     string
	serviceName string
}

// NewRestartPhp creates a new RestartPhp task
func NewRestartPhp(server *models.Server, version string) *RestartPhp {
	task := &RestartPhp{
		BaseServerTask: BaseServerTask{
			BaseTask: taskrunner.BaseTask{
				TemplateName: "server/php/restart",
				TaskTimeout:  30 * time.Second,
			},
			server: server,
		},
		version:     version,
		serviceName: fmt.Sprintf("php%s-fpm", version),
	}

	task.FinishedCallback = task.onFinished
	task.FailedCallback = task.onFailed
	task.TimeoutCallback = task.onTimeout

	return task
}

// Data returns the template data
func (t *RestartPhp) Data() map[string]interface{} {
	return map[string]interface{}{
		"Server":      t.server,
		"Version":     t.version,
		"PhpVersion":  t.version,
		"ServiceName": t.serviceName,
	}
}

// Version returns the PHP version
func (t *RestartPhp) Version() string {
	return t.version
}

// ServiceName returns the systemd service name
func (t *RestartPhp) ServiceName() string {
	return t.serviceName
}

// onFinished handles successful completion
func (t *RestartPhp) onFinished(ctx context.Context, result *taskrunner.TaskResult) {
	// PHP-FPM restarted successfully
	// Log success if needed
}

// onFailed handles task failure
func (t *RestartPhp) onFailed(ctx context.Context, result *taskrunner.TaskResult) {
	// Log the failure
	// Notify if needed
}

// onTimeout handles task timeout
func (t *RestartPhp) onTimeout(ctx context.Context, result *taskrunner.TaskResult) {
	// Log the timeout
	// Service restart should not normally timeout
}
