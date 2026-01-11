package tasks

import (
	"time"

	"github.com/kkz6/launch-go/internal/taskrunner"
)

// LaunchDaemonStatus checks the status of the Launch Agent daemon
type LaunchDaemonStatus struct {
	taskrunner.BaseTask
}

// NewLaunchDaemonStatus creates a new LaunchDaemonStatus task
func NewLaunchDaemonStatus() *LaunchDaemonStatus {
	task := &LaunchDaemonStatus{
		BaseTask: taskrunner.BaseTask{
			TaskName:     "launch-daemon-status",
			TemplateName: "launch-daemon-status",
			TaskTimeout:  30 * time.Second,
		},
	}

	return task
}

// Data returns the template data
func (t *LaunchDaemonStatus) Data() map[string]interface{} {
	return map[string]interface{}{}
}

// Script returns the command to run
func (t *LaunchDaemonStatus) Script() (string, error) {
	return "launch-agent status", nil
}
