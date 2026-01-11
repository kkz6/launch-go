package tasks

import (
	"fmt"
	"time"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// UninstallDaemon deletes a supervisor daemon configuration
type UninstallDaemon struct {
	BaseServerTask
	daemon *models.Daemon
}

// NewUninstallDaemon creates a new UninstallDaemon task
func NewUninstallDaemon(server *models.Server, daemon *models.Daemon) *UninstallDaemon {
	return &UninstallDaemon{
		BaseServerTask: BaseServerTask{
			BaseTask: taskrunner.BaseTask{
				TemplateName: "server/files/delete-file",
				TaskTimeout:  10 * time.Second,
			},
			server: server,
		},
		daemon: daemon,
	}
}

// Data returns the template data
func (t *UninstallDaemon) Data() map[string]interface{} {
	return map[string]interface{}{
		"Server":        t.server,
		"Path":          t.Path(),
		"IgnoreMissing": true,
	}
}

// Path returns the supervisor config file path to delete
func (t *UninstallDaemon) Path() string {
	return fmt.Sprintf("/etc/supervisor/conf.d/daemon-%s.conf", t.daemon.ID)
}

// ProgramName returns the supervisor program name
func (t *UninstallDaemon) ProgramName() string {
	return fmt.Sprintf("daemon-%s", t.daemon.ID)
}
