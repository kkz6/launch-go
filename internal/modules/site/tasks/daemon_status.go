package tasks

import (
	"github.com/kkz6/launch-go/internal/pkg/launch/status"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// Task type constants for daemon status
const (
	CheckDaemonStatusTaskType = "site:check_daemon_status"
)

// CheckDaemonStatus creates a task to check the status of all supervisor daemons.
// This delegates to the shared implementation in pkg/tasks.
func CheckDaemonStatus() *taskrunner.BaseTask {
	return status.CheckDaemonStatus()
}

// DaemonStatus is an alias for the shared type
type DaemonStatus = status.DaemonStatus

// ParseDaemonStatusOutput parses the JSON output from CheckDaemonStatus task.
// This delegates to the shared implementation in pkg/tasks.
func ParseDaemonStatusOutput(output string) []DaemonStatus {
	return status.ParseDaemonStatusOutput(output)
}

// FormatUptime converts seconds to a human-readable uptime string.
// This delegates to the shared implementation in launch/status.
func FormatUptime(seconds int) string {
	return status.FormatDaemonUptime(seconds)
}
