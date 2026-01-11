package tasks

import (
	"fmt"
	"time"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// UninstallCron deletes a cron job configuration file
type UninstallCron struct {
	BaseServerTask
	cron *models.Cron
}

// NewUninstallCron creates a new UninstallCron task
func NewUninstallCron(server *models.Server, cron *models.Cron) *UninstallCron {
	return &UninstallCron{
		BaseServerTask: BaseServerTask{
			BaseTask: taskrunner.BaseTask{
				TemplateName: "server/files/delete-file",
				TaskTimeout:  10 * time.Second,
			},
			server: server,
		},
		cron: cron,
	}
}

// Data returns the template data
func (t *UninstallCron) Data() map[string]interface{} {
	return map[string]interface{}{
		"Server":        t.server,
		"Path":          t.Path(),
		"IgnoreMissing": true,
	}
}

// Path returns the cron file path to delete
func (t *UninstallCron) Path() string {
	return fmt.Sprintf("/etc/cron.d/cron-%s", t.cron.ID)
}
