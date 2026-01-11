package tasks

import (
	"fmt"
	"time"

	"github.com/kkz6/launch-go/internal/taskrunner"
)

// RestartDaemon restarts a supervisor daemon by its ID
type RestartDaemon struct {
	taskrunner.BaseTask
	daemonID string
}

// NewRestartDaemon creates a new RestartDaemon task
func NewRestartDaemon(daemonID string) *RestartDaemon {
	task := &RestartDaemon{
		BaseTask: taskrunner.BaseTask{
			TaskName:     "restart-daemon",
			TemplateName: "restart-daemon",
			TaskTimeout:  30 * time.Second,
		},
		daemonID: daemonID,
	}

	return task
}

// Data returns the template data
func (t *RestartDaemon) Data() map[string]interface{} {
	return map[string]interface{}{
		"DaemonID": t.daemonID,
	}
}

// Script returns the command to run
func (t *RestartDaemon) Script() (string, error) {
	return fmt.Sprintf("sudo supervisorctl restart %s:", t.daemonID), nil
}

// DaemonID returns the daemon ID being restarted
func (t *RestartDaemon) DaemonID() string {
	return t.daemonID
}
