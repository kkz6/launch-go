package tasks

import (
	"time"

	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// Whoami runs the whoami command to verify SSH connectivity
type Whoami struct {
	taskrunner.BaseTask
}

// NewWhoami creates a new Whoami task
func NewWhoami() *Whoami {
	task := &Whoami{
		BaseTask: taskrunner.BaseTask{
			TaskName:     "whoami",
			TemplateName: "whoami",
			TaskTimeout:  15 * time.Second,
		},
	}

	return task
}

// Data returns the template data
func (t *Whoami) Data() map[string]interface{} {
	return map[string]interface{}{}
}

// Script returns the command to run directly without template
func (t *Whoami) Script() (string, error) {
	return "whoami", nil
}
