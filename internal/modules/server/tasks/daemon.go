package tasks

import (
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// UploadDaemonConfig holds configuration for uploading a daemon config
type UploadDaemonConfig struct {
	Path         string
	Contents     string
	LogPath      string
	ErrorLogPath string
	User         string
}

// UploadDaemon creates a task to upload a daemon config file to the server
func UploadDaemon(config UploadDaemonConfig) *taskrunner.BaseTask {
	script := `#!/bin/bash
set -euo pipefail

cat > "` + config.Path + `" << 'DAEMONEOF'
` + config.Contents + `
DAEMONEOF

chmod 644 "` + config.Path + `"

# Create .launch directory and log files if they don't exist
LOG_DIR=$(dirname "` + config.LogPath + `")
mkdir -p "$LOG_DIR"

if [ ! -f "` + config.LogPath + `" ]; then
    touch "` + config.LogPath + `"
    chown ` + config.User + `:` + config.User + ` "` + config.LogPath + `"
    chmod 644 "` + config.LogPath + `"
fi

if [ ! -f "` + config.ErrorLogPath + `" ]; then
    touch "` + config.ErrorLogPath + `"
    chown ` + config.User + `:` + config.User + ` "` + config.ErrorLogPath + `"
    chmod 644 "` + config.ErrorLogPath + `"
fi

# Ensure directory ownership
chown ` + config.User + `:` + config.User + ` "$LOG_DIR"

echo "Daemon config uploaded successfully"
`
	return taskrunner.NewBaseTask(
		taskrunner.WithName("Upload Daemon Config"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeout(30),
	)
}

// DeleteDaemonConfig holds configuration for deleting a daemon config
type DeleteDaemonConfig struct {
	Path        string
	ProgramName string
}

// DeleteDaemon creates a task to stop and delete a daemon from the server
func DeleteDaemon(config DeleteDaemonConfig) *taskrunner.BaseTask {
	script := `#!/bin/bash
set -euo pipefail

# Stop the daemon process
supervisorctl stop "` + config.ProgramName + `":* 2>/dev/null || true

# Remove the config file
if [ -f "` + config.Path + `" ]; then
    rm -f "` + config.Path + `"
fi

# Reload supervisor
supervisorctl reread
supervisorctl update

echo "Daemon removed successfully"
`
	return taskrunner.NewBaseTask(
		taskrunner.WithName("Delete Daemon"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeout(60),
	)
}

// RestartDaemonConfig holds configuration for restarting a daemon
type RestartDaemonConfig struct {
	ProgramName string
}

// RestartDaemon creates a task to restart a daemon on the server
func RestartDaemon(config RestartDaemonConfig) *taskrunner.BaseTask {
	script := `#!/bin/bash
set -euo pipefail

supervisorctl restart "` + config.ProgramName + `":*
echo "Daemon restarted successfully"
`
	return taskrunner.NewBaseTask(
		taskrunner.WithName("Restart Daemon"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeout(60),
	)
}

// ReloadSupervisor creates a task to reload supervisor configuration
func ReloadSupervisor() *taskrunner.BaseTask {
	script := `#!/bin/bash
set -euo pipefail

supervisorctl reread
supervisorctl update
echo "Supervisor reloaded successfully"
`
	return taskrunner.NewBaseTask(
		taskrunner.WithName("Reload Supervisor"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeout(30),
	)
}
