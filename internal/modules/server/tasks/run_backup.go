package tasks

import (
	"fmt"
	"time"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// RunBackup executes a backup operation via the launch-agent
type RunBackup struct {
	BaseServerTask
	backupID string
}

// NewRunBackup creates a new RunBackup task
func NewRunBackup(server *models.Server, backupID string) *RunBackup {
	task := &RunBackup{
		BaseServerTask: BaseServerTask{
			BaseTask: taskrunner.BaseTask{
				TemplateName: "server/files/run-backup",
				TaskTimeout:  10 * time.Minute,
			},
			server: server,
		},
		backupID: backupID,
	}

	return task
}

// Data returns the template data
func (t *RunBackup) Data() map[string]interface{} {
	return map[string]interface{}{
		"Server":   t.server,
		"BackupID": t.backupID,
		"Command":  t.buildCommand(),
	}
}

// BackupID returns the backup ID to execute
func (t *RunBackup) BackupID() string {
	return t.backupID
}

// buildCommand constructs the shell command for running the backup
func (t *RunBackup) buildCommand() string {
	return fmt.Sprintf("launch-agent perform -m %s", t.backupID)
}
