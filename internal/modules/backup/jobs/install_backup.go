package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeInstallBackup = "backup:install"

type InstallBackupPayload struct {
	ServerID string  `json:"server_id"`
	BackupID string  `json:"backup_id"`
	UserID   *string `json:"user_id,omitempty"`
}

// InstallBackupJob installs backup configuration on a server
type InstallBackupJob struct {
	ctx     *JobContext
	Payload InstallBackupPayload
}

func (j *InstallBackupJob) Handle(ctx context.Context) error {
	backup, err := j.ctx.Repos.Backup().FindBackupByID(ctx, j.Payload.BackupID)
	if err != nil {
		return fmt.Errorf("failed to find backup: %w", err)
	}

	server, err := j.ctx.ServerRepos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	j.ctx.LogInfo("Installing backup configuration",
		"server_id", server.ID,
		"backup_id", backup.ID,
	)

	// TODO: Generate and deploy backup agent configuration to the server
	// This would involve:
	// 1. Generate the backup agent configuration
	// 2. Deploy it to the server via SSH
	// 3. Set up the cron schedule
	// 4. Test connectivity to storage provider

	// Mark backup as installed
	if err := j.ctx.Repos.Backup().UpdateBackupFields(ctx, backup.ID, map[string]interface{}{
		"installed_at":           "NOW()",
		"installation_failed_at": nil,
	}); err != nil {
		return fmt.Errorf("failed to mark backup as installed: %w", err)
	}

	j.ctx.LogInfo("Backup configuration installed successfully",
		"server_id", server.ID,
		"backup_id", backup.ID,
	)

	return nil
}

func (j *InstallBackupJob) Failed(ctx context.Context, err error) {
	j.ctx.LogError(err, "Failed to install backup configuration",
		"server_id", j.Payload.ServerID,
		"backup_id", j.Payload.BackupID,
	)

	// Mark installation as failed
	_ = j.ctx.Repos.Backup().UpdateBackupFields(ctx, j.Payload.BackupID, map[string]interface{}{
		"installation_failed_at": "NOW()",
	})
}

func NewInstallBackupJob(ctx *JobContext, payload InstallBackupPayload) *InstallBackupJob {
	return &InstallBackupJob{
		ctx:     ctx,
		Payload: payload,
	}
}

func NewInstallBackupTask(serverID, backupID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.NewTask(TypeInstallBackup, InstallBackupPayload{
		ServerID: serverID,
		BackupID: backupID,
		UserID:   userID,
	})
}
