package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/backup/models"
	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeInstallBackup = "backup:install"

// InstallBackupJob installs backup configuration on a server.
type InstallBackupJob struct {
	Deps    *JobDeps
	Payload InstallBackupPayload

	server *servermodels.Server
	backup *models.Backup
}

func NewInstallBackupJob(p InstallBackupPayload) pkgjobs.Handler {
	return &InstallBackupJob{Deps: deps, Payload: p}
}

func (j *InstallBackupJob) Handle(ctx context.Context) error {
	var err error

	j.backup, err = j.Deps.Repos.Backup().FindBackupByID(ctx, j.Payload.BackupID)
	if err != nil {
		return fmt.Errorf("find backup: %w", err)
	}

	j.server, err = j.Deps.ServerRepos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("find server: %w", err)
	}

	j.Deps.Logger.Info().
		Str("server_id", j.server.ID).
		Str("backup_id", j.backup.ID).
		Msg("installing backup configuration")

	return fmt.Errorf("backup agent installation is not implemented")
}

func (j *InstallBackupJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).
		Str("server_id", j.Payload.ServerID).
		Str("backup_id", j.Payload.BackupID).
		Msg("failed to install backup configuration")

	// Mark installation as failed
	if updateErr := j.Deps.Repos.Backup().UpdateBackupFields(ctx, j.Payload.BackupID, map[string]interface{}{
		"installation_failed_at": "NOW()",
	}); updateErr != nil {
		j.Deps.Logger.Error().Err(updateErr).Str("backup_id", j.Payload.BackupID).Msg("failed to persist backup installation failure")
	}
}

func NewInstallBackupTask(serverID, backupID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.TaskWithID(TypeInstallBackup,
		InstallBackupPayload{ServerID: serverID, BackupID: backupID, UserID: userID},
		pkgjobs.Dedup("backup-install", serverID, backupID),
	)
}
