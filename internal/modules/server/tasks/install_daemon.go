package tasks

import (
	"fmt"
	"time"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// InstallDaemon creates and uploads a supervisor daemon configuration
type InstallDaemon struct {
	BaseServerTask
	daemon *models.Daemon
}

// NewInstallDaemon creates a new InstallDaemon task
func NewInstallDaemon(server *models.Server, daemon *models.Daemon) *InstallDaemon {
	return &InstallDaemon{
		BaseServerTask: BaseServerTask{
			BaseTask: taskrunner.BaseTask{
				TemplateName: "server/files/upload-file",
				TaskTimeout:  20 * time.Second,
			},
			server: server,
		},
		daemon: daemon,
	}
}

// Data returns the template data
func (t *InstallDaemon) Data() map[string]interface{} {
	return map[string]interface{}{
		"Server":    t.server,
		"Path":      t.Path(),
		"Contents":  t.Contents(),
		"IsRoot":    true,
		"Directory": "/etc/supervisor/conf.d",
	}
}

// Path returns the supervisor config file path
func (t *InstallDaemon) Path() string {
	return fmt.Sprintf("/etc/supervisor/conf.d/daemon-%s.conf", t.daemon.ID)
}

// ProgramName returns the supervisor program name
func (t *InstallDaemon) ProgramName() string {
	return fmt.Sprintf("daemon-%s", t.daemon.ID)
}

// Contents builds the supervisor program configuration
func (t *InstallDaemon) Contents() string {
	content := fmt.Sprintf(`[program:%s]
process_name=%%(program_name)s_%%(process_num)02d
command=%s
user=%s
`, t.daemon.ID, t.daemon.Command, t.daemon.User)

	if t.daemon.Directory != nil && *t.daemon.Directory != "" {
		content += fmt.Sprintf("directory=%s\n", *t.daemon.Directory)
	}

	content += fmt.Sprintf(`autostart=true
autorestart=true
numprocs=%d
startsecs=1
stopsignal=%s
stopwaitsecs=%d
stopasgroup=true
killasgroup=true
stdout_logfile=%s
stderr_logfile=%s
stdout_logfile_maxbytes=5MB
stderr_logfile_maxbytes=5MB
redirect_stderr=true
`, t.daemon.Processes, t.daemon.StopSignal, t.daemon.StopWaitSeconds, t.getLogPath(), t.getErrorLogPath())

	return content
}

// getLogPath returns the path to the output log file
func (t *InstallDaemon) getLogPath() string {
	if t.daemon.User == "root" {
		return fmt.Sprintf("/root/daemon-%s.log", t.daemon.ID)
	}
	return fmt.Sprintf("/home/%s/daemon-%s.log", t.daemon.User, t.daemon.ID)
}

// getErrorLogPath returns the path to the error log file
func (t *InstallDaemon) getErrorLogPath() string {
	if t.daemon.User == "root" {
		return fmt.Sprintf("/root/daemon-%s.err", t.daemon.ID)
	}
	return fmt.Sprintf("/home/%s/daemon-%s.err", t.daemon.User, t.daemon.ID)
}
