package tasks

import (
	"context"
	"fmt"
	"time"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// UpdateAlternatives updates the system alternatives configuration
type UpdateAlternatives struct {
	BaseServerTask
	link string
	path string
}

// NewUpdateAlternatives creates a new UpdateAlternatives task
func NewUpdateAlternatives(server *models.Server, link string, path string) *UpdateAlternatives {
	task := &UpdateAlternatives{
		BaseServerTask: BaseServerTask{
			BaseTask: taskrunner.BaseTask{
				TemplateName: "server/php/update-alternatives",
				TaskTimeout:  30 * time.Second,
			},
			server: server,
		},
		link: link,
		path: path,
	}

	task.FinishedCallback = task.onFinished
	task.FailedCallback = task.onFailed
	task.TimeoutCallback = task.onTimeout

	return task
}

// NewUpdatePhpAlternatives creates a task to update the default PHP version
func NewUpdatePhpAlternatives(server *models.Server, version string) *UpdateAlternatives {
	return NewUpdateAlternatives(
		server,
		"php",
		fmt.Sprintf("/usr/bin/php%s", version),
	)
}

// Data returns the template data
func (t *UpdateAlternatives) Data() map[string]interface{} {
	return map[string]interface{}{
		"Server": t.server,
		"Link":   t.link,
		"Path":   t.path,
	}
}

// Link returns the alternative link name
func (t *UpdateAlternatives) Link() string {
	return t.link
}

// Path returns the alternative path
func (t *UpdateAlternatives) Path() string {
	return t.path
}

// Command returns the command that will be executed
func (t *UpdateAlternatives) Command() string {
	return fmt.Sprintf("update-alternatives --set %s %s", t.link, t.path)
}

// onFinished handles successful completion
func (t *UpdateAlternatives) onFinished(ctx context.Context, result *taskrunner.TaskResult) {
	// Alternatives updated successfully
	// Log success if needed
}

// onFailed handles task failure
func (t *UpdateAlternatives) onFailed(ctx context.Context, result *taskrunner.TaskResult) {
	// Log the failure
	// Notify if needed
}

// onTimeout handles task timeout
func (t *UpdateAlternatives) onTimeout(ctx context.Context, result *taskrunner.TaskResult) {
	// Log the timeout
	// This should not normally timeout
}
