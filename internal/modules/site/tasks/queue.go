package tasks

import (
	"fmt"

	"github.com/kkz6/launch-go/internal/modules/site/formatter"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// Task type constants for queue operations
const (
	UploadQueueConfigTaskType = "site:upload_queue_config"
	DeleteQueueConfigTaskType = "site:delete_queue_config"
	RestartQueueTaskType      = "site:restart_queue"
	RestartAllQueuesTaskType  = "site:restart_all_queues"
)

// BuildQueueSupervisorConfig generates the supervisor config for a queue worker
func BuildQueueSupervisorConfig(queue *models.Queue, serverUsername string) string {
	directory := ""
	if queue.Directory != nil {
		directory = *queue.Directory
	}

	return fmt.Sprintf(`[program:%s]
command=%s

process_name=%%(program_name)s_%%(process_num)02d
autostart=true
autorestart=true
stopasgroup=true
killasgroup=true
user=%s
directory=%s
numprocs=%d
stdout_logfile=%s
stderr_logfile=%s
stdout_logfile_maxbytes=5MB
`, queue.ID, formatter.QueueCommand(queue), serverUsername, directory, queue.NumProcs, queue.GetLogPath(), queue.GetErrorLogPath())
}

// UploadQueueConfigParams holds parameters for uploading queue config
type UploadQueueConfigParams struct {
	Path         string
	Contents     string
	LogPath      string
	ErrorLogPath string
	User         string
	WorkingDir   string
}

// UploadQueueConfig creates a task to upload a queue supervisor config
func UploadQueueConfig(params UploadQueueConfigParams) *taskrunner.BaseTask {
	script := `#!/bin/bash
set -euo pipefail

cat > "` + params.Path + `" << 'QUEUEEOF'
` + params.Contents + `
QUEUEEOF

chmod 644 "` + params.Path + `"

# Create .launch directory and log files if they don't exist
LOG_DIR=$(dirname "` + params.LogPath + `")
mkdir -p "$LOG_DIR"

if [ ! -f "` + params.LogPath + `" ]; then
    touch "` + params.LogPath + `"
    chown ` + params.User + `:` + params.User + ` "` + params.LogPath + `"
    chmod 644 "` + params.LogPath + `"
fi

if [ ! -f "` + params.ErrorLogPath + `" ]; then
    touch "` + params.ErrorLogPath + `"
    chown ` + params.User + `:` + params.User + ` "` + params.ErrorLogPath + `"
    chmod 644 "` + params.ErrorLogPath + `"
fi

# Ensure directory ownership
chown ` + params.User + `:` + params.User + ` "$LOG_DIR"

echo "Queue config uploaded successfully"
`
	return taskrunner.NewBaseTask(
		taskrunner.WithName("Upload Queue Config"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(30),
	)
}

// DeleteQueueConfig creates a task to stop and delete a queue from the server
func DeleteQueueConfig(configPath, programName string) *taskrunner.BaseTask {
	script := `#!/bin/bash
set -euo pipefail

# Stop the queue process
supervisorctl stop "` + programName + `":* 2>/dev/null || true

# Remove the config file
if [ -f "` + configPath + `" ]; then
    rm -f "` + configPath + `"
fi

# Reload supervisor
supervisorctl reread
supervisorctl update

echo "Queue removed successfully"
`
	return taskrunner.NewBaseTask(
		taskrunner.WithName("Delete Queue Config"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(60),
	)
}

// RestartQueue creates a task to restart a queue worker
func RestartQueue(programName string) *taskrunner.BaseTask {
	script := `#!/bin/bash
set -euo pipefail

# Stop the process first (handles FATAL state from previous failed starts)
supervisorctl stop "` + programName + `":* 2>/dev/null || true

# Reread and update configs in case they changed
supervisorctl reread
supervisorctl update

# Start the process
supervisorctl start "` + programName + `":*
echo "Queue restarted successfully"
`
	return taskrunner.NewBaseTask(
		taskrunner.WithName("Restart Queue"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(60),
	)
}

// RestartAllQueues creates a task to restart all queues for a site
func RestartAllQueues(queueIDs []string) *taskrunner.BaseTask {
	script := `#!/bin/bash
set -euo pipefail

# Stop all queue processes first (handles FATAL state from previous failed starts)
`
	for _, id := range queueIDs {
		script += fmt.Sprintf(`supervisorctl stop "%s":* 2>/dev/null || true
`, id)
	}

	script += `
# Reread and update configs
supervisorctl reread
supervisorctl update

# Start all queue processes
`
	script += "restart_failed=0\n"
	for _, id := range queueIDs {
		script += fmt.Sprintf(`if ! supervisorctl start "%s":*; then
  echo "Queue %s failed to start" >&2
  restart_failed=1
fi
`, id, id)
	}

	script += `
if [ "$restart_failed" -ne 0 ]; then
  exit 1
fi

echo "All queues restarted"
`
	return taskrunner.NewBaseTask(
		taskrunner.WithName("Restart All Queues"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(120),
	)
}
