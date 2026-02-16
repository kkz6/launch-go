package tasks

import (
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
	"github.com/kkz6/launch-go/internal/pkg/util"
)

// Task type constants for daemon operations
const (
	UploadDaemonTaskType     = "server:upload_daemon"
	DeleteDaemonTaskType     = "server:delete_daemon"
	RestartDaemonTaskType    = "server:restart_daemon"
	ReloadSupervisorTaskType = "server:reload_supervisor"
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
	configScript := util.WriteConfig(util.WriteConfigParams{
		FilePath:     config.Path,
		Content:      config.Contents,
		User:         config.User,
		LogPath:      config.LogPath,
		ErrorLogPath: config.ErrorLogPath,
		Message:      "Daemon config uploaded successfully",
	})

	return taskrunner.NewBaseTask(
		taskrunner.WithName("Upload Daemon Config"),
		taskrunner.WithScript(configScript),
		taskrunner.WithTimeoutSeconds(30),
	)
}

// DeleteDaemonConfig holds configuration for deleting a daemon config
type DeleteDaemonConfig struct {
	Path        string
	ProgramName string
}

// DeleteDaemon creates a task to stop and delete a daemon from the server
func DeleteDaemon(config DeleteDaemonConfig) *taskrunner.BaseTask {
	deleteScript := `#!/bin/bash
set -euo pipefail

# Stop the daemon process
sudo supervisorctl stop "` + config.ProgramName + `":* 2>/dev/null || true

# Remove the config file
if [ -f "` + config.Path + `" ]; then
    sudo rm -f "` + config.Path + `"
fi

# Reload supervisor
sudo supervisorctl reread
sudo supervisorctl update

echo "Daemon removed successfully"
`
	return taskrunner.NewBaseTask(
		taskrunner.WithName("Delete Daemon"),
		taskrunner.WithScript(deleteScript),
		taskrunner.WithTimeoutSeconds(60),
	)
}

// RestartDaemonConfig holds configuration for restarting a daemon
type RestartDaemonConfig struct {
	ProgramName string
}

// RestartDaemon creates a task to restart a daemon on the server
func RestartDaemon(config RestartDaemonConfig) *taskrunner.BaseTask {
	restartScript := `#!/bin/bash
set -euo pipefail

sudo supervisorctl restart "` + config.ProgramName + `":*
echo "Daemon restarted successfully"
`
	return taskrunner.NewBaseTask(
		taskrunner.WithName("Restart Daemon"),
		taskrunner.WithScript(restartScript),
		taskrunner.WithTimeoutSeconds(60),
	)
}

// ReloadSupervisor creates a task to reload supervisor configuration
func ReloadSupervisor() *taskrunner.BaseTask {
	reloadScript := `#!/bin/bash
set -euo pipefail

sudo supervisorctl reread
sudo supervisorctl update
echo "Supervisor reloaded successfully"
`
	return taskrunner.NewBaseTask(
		taskrunner.WithName("Reload Supervisor"),
		taskrunner.WithScript(reloadScript),
		taskrunner.WithTimeoutSeconds(30),
	)
}
