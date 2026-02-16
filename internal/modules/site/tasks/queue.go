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

sudo tee "` + params.Path + `" > /dev/null << 'QUEUEEOF'
` + params.Contents + `
QUEUEEOF

sudo chmod 644 "` + params.Path + `"

# Create .launch directory and log files if they don't exist
LOG_DIR=$(dirname "` + params.LogPath + `")
mkdir -p "$LOG_DIR"

if [ ! -f "` + params.LogPath + `" ]; then
    touch "` + params.LogPath + `"
fi

if [ ! -f "` + params.ErrorLogPath + `" ]; then
    touch "` + params.ErrorLogPath + `"
fi

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
sudo supervisorctl stop "` + programName + `":* 2>/dev/null || true

# Remove the config file
if [ -f "` + configPath + `" ]; then
    sudo rm -f "` + configPath + `"
fi

# Reload supervisor
sudo supervisorctl reread
sudo supervisorctl update

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

sudo supervisorctl restart "` + programName + `":*
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

`
	for _, id := range queueIDs {
		script += fmt.Sprintf(`sudo supervisorctl restart "%s":* 2>/dev/null || echo "Queue %s not found or already stopped"
`, id, id)
	}

	script += `
echo "All queues restarted"
`
	return taskrunner.NewBaseTask(
		taskrunner.WithName("Restart All Queues"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(120),
	)
}
