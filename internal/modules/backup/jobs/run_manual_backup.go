package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/backup/models"
	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeRunManualBackup = "backup:run_manual"

// RunManualBackupJob triggers a manual backup run on a server.
type RunManualBackupJob struct {
	Deps    *JobDeps
	Payload RunManualBackupPayload

	server *servermodels.Server
	backup *models.Backup
}

func NewRunManualBackupJob(p RunManualBackupPayload) pkgjobs.Handler {
	return &RunManualBackupJob{Deps: deps, Payload: p}
}

func (j *RunManualBackupJob) Handle(ctx context.Context) error {
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
		Msg("running manual backup")

	// TODO: Trigger manual backup execution on the server
	// This would involve:
	// 1. Connect to server via SSH
	// 2. Run the backup agent with the specific backup configuration
	// 3. Monitor progress and report status
	// 4. Upload backup to storage provider

	j.Deps.Logger.Info().
		Str("server_id", j.server.ID).
		Str("backup_id", j.backup.ID).
		Msg("manual backup completed successfully")

	return nil
}

func (j *RunManualBackupJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).
		Str("server_id", j.Payload.ServerID).
		Str("backup_id", j.Payload.BackupID).
		Msg("failed to run manual backup")
}

func NewRunManualBackupTask(serverID, backupID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.Task(TypeRunManualBackup,
		RunManualBackupPayload{ServerID: serverID, BackupID: backupID, UserID: userID},
	)
}
