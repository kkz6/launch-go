package tasks

import (
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner/templates"
)

func newDatabaseTask(name, templateName string, config any, timeoutSeconds int) *taskrunner.BaseTask {
	return taskrunner.NewBaseTask(
		taskrunner.WithName(name),
		taskrunner.WithScript(templates.MustRender("database", templateName, config)),
		taskrunner.WithTimeoutSeconds(timeoutSeconds),
	)
}
