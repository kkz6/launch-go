package tasks

import (
	"time"

	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// ReloadSupervisor reloads Supervisor configuration (reread and update)
type ReloadSupervisor struct {
	taskrunner.BaseTask
}

// NewReloadSupervisor creates a new ReloadSupervisor task
func NewReloadSupervisor() *ReloadSupervisor {
	task := &ReloadSupervisor{
		BaseTask: taskrunner.BaseTask{
			TaskName:     "reload-supervisor",
			TemplateName: "server/reload-supervisor",
			TaskTimeout:  2 * time.Minute,
		},
	}

	return task
}

// Data returns the template data
func (t *ReloadSupervisor) Data() map[string]interface{} {
	return map[string]interface{}{}
}

// Script returns the commands to run
func (t *ReloadSupervisor) Script() (string, error) {
	return `sudo supervisorctl reread
sudo supervisorctl update`, nil
}
