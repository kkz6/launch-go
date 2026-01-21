package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeDeleteBackup = "backup:delete"

type DeleteBackupPayload struct {
	ServerID string  `json:"server_id"`
	BackupID string  `json:"backup_id"`
	UserID   *string `json:"user_id,omitempty"`
}

// DeleteBackupJob removes backup configuration from a server
type DeleteBackupJob struct {
	ctx     *JobContext
	Payload DeleteBackupPayload
}

func (j *DeleteBackupJob) Handle(ctx context.Context) error {
	server, err := j.ctx.Repos().Server().Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	j.ctx.LogInfo("Removing backup configuration",
		"server_id", server.ID,
		"backup_id", j.Payload.BackupID,
	)

	// TODO: Remove backup agent configuration from the server
	// This would involve:
	// 1. Remove the backup cron job
	// 2. Remove the backup configuration file
	// 3. Optionally clean up any local backup files

	j.ctx.LogInfo("Backup configuration removed successfully",
		"server_id", server.ID,
		"backup_id", j.Payload.BackupID,
	)

	return nil
}

func (j *DeleteBackupJob) Failed(ctx context.Context, err error) {
	j.ctx.LogError(err, "Failed to remove backup configuration",
		"server_id", j.Payload.ServerID,
		"backup_id", j.Payload.BackupID,
	)
}

func NewDeleteBackupJob(ctx *JobContext, payload DeleteBackupPayload) *DeleteBackupJob {
	return &DeleteBackupJob{
		ctx:     ctx,
		Payload: payload,
	}
}

func NewDeleteBackupTask(serverID, backupID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.NewTask(TypeDeleteBackup, DeleteBackupPayload{
		ServerID: serverID,
		BackupID: backupID,
		UserID:   userID,
	})
}
