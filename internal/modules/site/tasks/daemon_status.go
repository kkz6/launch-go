package tasks

import (
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
	pkgtasks "github.com/kkz6/launch-go/internal/pkg/tasks"
)

// Task type constants for daemon status
const (
	CheckDaemonStatusTaskType = "site:check_daemon_status"
)

// CheckDaemonStatus creates a task to check the status of all supervisor daemons.
// This delegates to the shared implementation in pkg/tasks.
func CheckDaemonStatus() *taskrunner.BaseTask {
	return pkgtasks.CheckDaemonStatus()
}

// DaemonStatus is an alias for the shared type
type DaemonStatus = pkgtasks.DaemonStatus

// ParseDaemonStatusOutput parses the JSON output from CheckDaemonStatus task.
// This delegates to the shared implementation in pkg/tasks.
func ParseDaemonStatusOutput(output string) []DaemonStatus {
	return pkgtasks.ParseDaemonStatusOutput(output)
}

// FormatUptime converts seconds to a human-readable uptime string.
// This delegates to the shared implementation in pkg/tasks.
func FormatUptime(seconds int) string {
	return pkgtasks.FormatUptime(seconds)
}
