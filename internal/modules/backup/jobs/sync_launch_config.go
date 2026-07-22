package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeSyncServerLaunchConfig = "backup:sync_launch_config"

// SyncServerLaunchConfigPayload holds data for syncing launch config to server.
type SyncServerLaunchConfigPayload struct {
	ServerID string  `json:"server_id"`
	UserID   *string `json:"user_id,omitempty"`
}

// SyncServerLaunchConfigJob syncs the Launch backup agent configuration to a server.
type SyncServerLaunchConfigJob struct {
	Deps    *JobDeps
	Payload SyncServerLaunchConfigPayload

	server *servermodels.Server
}

func NewSyncServerLaunchConfigJob(p SyncServerLaunchConfigPayload) pkgjobs.Handler {
	return &SyncServerLaunchConfigJob{Deps: deps, Payload: p}
}

func (j *SyncServerLaunchConfigJob) Handle(ctx context.Context) error {
	var err error

	j.server, err = j.Deps.ServerRepos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("find server: %w", err)
	}

	j.Deps.Logger.Info().
		Str("server_id", j.server.ID).
		Msg("syncing Launch backup configuration to server")

	// Get all backups for this server
	backups, err := j.Deps.Repos.Backup().FindBackupsByServerID(ctx, j.server.ID)
	if err != nil {
		return fmt.Errorf("get backups for server: %w", err)
	}

	// Count total backup jobs across all backups
	var totalBackupJobs int
	for _, backup := range backups {
		backupJobs, err := j.Deps.Repos.BackupJob().FindBackupJobsByBackupID(ctx, backup.ID)
		if err != nil {
			j.Deps.Logger.Error().Err(err).
				Str("backup_id", backup.ID).
				Msg("failed to get backup jobs")
			continue
		}
		totalBackupJobs += len(backupJobs)
	}

	j.Deps.Logger.Info().
		Str("server_id", j.server.ID).
		Int("backups_count", len(backups)).
		Int("backup_jobs_count", totalBackupJobs).
		Msg("found backup configurations")

	return fmt.Errorf("backup agent configuration sync is not implemented")
}

func (j *SyncServerLaunchConfigJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).
		Str("server_id", j.Payload.ServerID).
		Msg("failed to sync Launch backup configuration")
}

func NewSyncServerLaunchConfigTask(serverID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.TaskWithID(TypeSyncServerLaunchConfig,
		SyncServerLaunchConfigPayload{ServerID: serverID, UserID: userID},
		pkgjobs.Dedup("backup-sync-config", serverID),
	)
}
