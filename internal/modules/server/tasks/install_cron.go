package tasks

import (
	"fmt"
	"time"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// InstallCron creates and uploads a cron job configuration file
type InstallCron struct {
	BaseServerTask
	cron *models.Cron
}

// NewInstallCron creates a new InstallCron task
func NewInstallCron(server *models.Server, cron *models.Cron) *InstallCron {
	return &InstallCron{
		BaseServerTask: BaseServerTask{
			BaseTask: taskrunner.BaseTask{
				TemplateName: "server/files/upload-file",
				TaskTimeout:  20 * time.Second,
			},
			server: server,
		},
		cron: cron,
	}
}

// Data returns the template data
func (t *InstallCron) Data() map[string]interface{} {
	return map[string]interface{}{
		"Server":    t.server,
		"Path":      t.Path(),
		"Contents":  t.Contents(),
		"IsRoot":    true,
		"Directory": "/etc/cron.d",
	}
}

// Path returns the cron file path
func (t *InstallCron) Path() string {
	return fmt.Sprintf("/etc/cron.d/cron-%s", t.cron.ID)
}

// Contents builds the cron file content for /etc/cron.d/
func (t *InstallCron) Contents() string {
	logPath := t.getLogPath()
	return fmt.Sprintf(`SHELL=/bin/sh
PATH=/usr/local/sbin:/usr/local/bin:/sbin:/bin:/usr/sbin:/usr/bin

%s %s %s > %s 2>&1
`, t.cron.Expression, t.cron.User, t.cron.Command, logPath)
}

// getLogPath returns the path to the cron log file
func (t *InstallCron) getLogPath() string {
	if t.cron.User == "root" {
		return fmt.Sprintf("/root/cron-%s.log", t.cron.ID)
	}
	return fmt.Sprintf("/home/%s/cron-%s.log", t.cron.User, t.cron.ID)
}
