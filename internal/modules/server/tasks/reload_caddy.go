package tasks

import (
	"time"

	"github.com/kkz6/launch-go/internal/taskrunner"
)

// ReloadCaddy reloads the Caddy web server configuration
type ReloadCaddy struct {
	taskrunner.BaseTask
}

// NewReloadCaddy creates a new ReloadCaddy task
func NewReloadCaddy() *ReloadCaddy {
	task := &ReloadCaddy{
		BaseTask: taskrunner.BaseTask{
			TaskName:     "reload-caddy",
			TemplateName: "reload-caddy",
			TaskTimeout:  30 * time.Second,
		},
	}

	return task
}

// Data returns the template data
func (t *ReloadCaddy) Data() map[string]interface{} {
	return map[string]interface{}{}
}

// Script returns the command to run directly
func (t *ReloadCaddy) Script() (string, error) {
	return "sudo /usr/sbin/service caddy reload", nil
}
