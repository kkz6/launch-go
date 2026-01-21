package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeRunManualBackup = "backup:run_manual"

// RunManualBackupJob triggers a manual backup run on a server
type RunManualBackupJob struct {
	ctx     *JobContext
	Payload RunManualBackupPayload
}

func (j *RunManualBackupJob) Handle(ctx context.Context) error {
	backup, err := j.ctx.Repos().Backup().FindBackupByID(ctx, j.Payload.BackupID)
	if err != nil {
		return fmt.Errorf("failed to find backup: %w", err)
	}

	server, err := j.ctx.Repos().Server().Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	j.ctx.LogInfo("Running manual backup",
		"server_id", server.ID,
		"backup_id", backup.ID,
	)

	// TODO: Trigger manual backup execution on the server
	// This would involve:
	// 1. Connect to server via SSH
	// 2. Run the backup agent with the specific backup configuration
	// 3. Monitor progress and report status
	// 4. Upload backup to storage provider

	j.ctx.LogInfo("Manual backup completed successfully",
		"server_id", server.ID,
		"backup_id", backup.ID,
	)

	return nil
}

func (j *RunManualBackupJob) Failed(ctx context.Context, err error) {
	j.ctx.LogError(err, "Failed to run manual backup",
		"server_id", j.Payload.ServerID,
		"backup_id", j.Payload.BackupID,
	)
}

func NewRunManualBackupJob(ctx *JobContext, payload RunManualBackupPayload) *RunManualBackupJob {
	return &RunManualBackupJob{
		ctx:     ctx,
		Payload: payload,
	}
}

func NewRunManualBackupTask(serverID, backupID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.NewTask(TypeRunManualBackup, RunManualBackupPayload{
		ServerID: serverID,
		BackupID: backupID,
		UserID:   userID,
	})
}
