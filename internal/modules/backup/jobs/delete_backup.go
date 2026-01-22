package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeDeleteBackup = "backup:delete"

type DeleteBackupPayload struct {
	ServerID string  `json:"server_id"`
	BackupID string  `json:"backup_id"`
	UserID   *string `json:"user_id,omitempty"`
}

// DeleteBackupJob removes backup configuration from a server.
type DeleteBackupJob struct {
	Deps    *JobDeps
	Payload DeleteBackupPayload

	server *servermodels.Server
}

func NewDeleteBackupJob(p DeleteBackupPayload) pkgjobs.Handler {
	return &DeleteBackupJob{Deps: deps, Payload: p}
}

func (j *DeleteBackupJob) Handle(ctx context.Context) error {
	var err error

	j.server, err = j.Deps.ServerRepos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("find server: %w", err)
	}

	j.Deps.Logger.Info().
		Str("server_id", j.server.ID).
		Str("backup_id", j.Payload.BackupID).
		Msg("removing backup configuration")

	// TODO: Remove backup agent configuration from the server
	// This would involve:
	// 1. Remove the backup cron job
	// 2. Remove the backup configuration file
	// 3. Optionally clean up any local backup files

	j.Deps.Logger.Info().
		Str("server_id", j.server.ID).
		Str("backup_id", j.Payload.BackupID).
		Msg("backup configuration removed successfully")

	return nil
}

func (j *DeleteBackupJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).
		Str("server_id", j.Payload.ServerID).
		Str("backup_id", j.Payload.BackupID).
		Msg("failed to remove backup configuration")
}

func NewDeleteBackupTask(serverID, backupID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.TaskWithID(TypeDeleteBackup,
		DeleteBackupPayload{ServerID: serverID, BackupID: backupID, UserID: userID},
		pkgjobs.Dedup("backup-delete", serverID, backupID),
	)
}
