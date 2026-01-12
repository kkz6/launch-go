package tasks

import (
	"fmt"
	"strings"
	"time"

	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// ServerTask extends BaseTask with server-specific functionality
type ServerTask struct {
	*taskrunner.BaseTask
}

// NewServerTask creates a new ServerTask with script
func NewServerTask(script string, opts ...taskrunner.TaskOption) *ServerTask {
	return &ServerTask{
		BaseTask: taskrunner.NewBaseTask(append([]taskrunner.TaskOption{
			taskrunner.WithScript(wrapScript(script)),
		}, opts...)...),
	}
}

// NewServerTaskWithName creates a new ServerTask with name and script
func NewServerTaskWithName(name string, script string, timeoutSeconds int) *ServerTask {
	return &ServerTask{
		BaseTask: taskrunner.NewBaseTask(
			taskrunner.WithName(name),
			taskrunner.WithScript(wrapScript(script)),
			taskrunner.WithTimeout(time.Duration(timeoutSeconds)*time.Second),
		),
	}
}

// wrapScript wraps a script with shell defaults
func wrapScript(script string) string {
	return fmt.Sprintf(`#!/bin/bash
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive

%s
`, strings.TrimSpace(script))
}

// Pending creates a PendingTask from this task
func (t *ServerTask) Pending() *taskrunner.PendingTask {
	return taskrunner.NewPendingTask(t)
}

// ShellDefaults returns the shell defaults header
func ShellDefaults() string {
	return `set -euo pipefail
export DEBIAN_FRONTEND=noninteractive
`
}

// CommonFunctions returns common bash functions for tasks
func CommonFunctions() string {
	return `# Send a POST request to the given URL, ignoring the response and errors
function httpPostSilently() {
    if [ -z "${2:-}" ]; then
        (curl -X POST --silent --max-time 15 --output /dev/null $1 || true)
    else
        (curl -X POST --silent --max-time 15 --output /dev/null $1 -H 'Content-Type: application/json' --data "$2" || true)
    fi
}

function httpPostRawSilently() {
    (curl -X POST --silent --max-time 15 --output /dev/null $1 --data "$2" || true)
}
`
}

// AptFunctions returns apt-related bash functions
func AptFunctions() string {
	return `# Wait for apt to be unlocked
function waitForAptUnlock() {
    while ps -C apt,apt-get,dpkg >/dev/null 2>&1; do
        echo "apt, apt-get or dpkg is running..."
        sleep 5
    done

    while fuser /var/{lib/{dpkg,apt/lists},cache/apt/archives}/{lock,lock-frontend} >/dev/null 2>&1; do
        echo "Waiting: apt is locked..."
        sleep 5
    done

    if [ -f /var/log/unattended-upgrades/unattended-upgrades.log ]; then
        while fuser /var/log/unattended-upgrades/unattended-upgrades.log >/dev/null 2>&1; do
            echo "Waiting: unattended-upgrades is locked..."
            sleep 5
        done
    fi
}
`
}
