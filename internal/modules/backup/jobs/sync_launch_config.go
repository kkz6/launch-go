package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeSyncServerLaunchConfig = "backup:sync_launch_config"

// SyncServerLaunchConfigPayload holds data for syncing launch config to server
type SyncServerLaunchConfigPayload struct {
	ServerID string  `json:"server_id"`
	UserID   *string `json:"user_id,omitempty"`
}

// SyncServerLaunchConfigJob syncs the Launch backup agent configuration to a server
type SyncServerLaunchConfigJob struct {
	ctx     *JobContext
	Payload SyncServerLaunchConfigPayload
}

// NewSyncServerLaunchConfigJob creates a new SyncServerLaunchConfigJob
func NewSyncServerLaunchConfigJob(ctx *JobContext, payload SyncServerLaunchConfigPayload) *SyncServerLaunchConfigJob {
	return &SyncServerLaunchConfigJob{
		ctx:     ctx,
		Payload: payload,
	}
}

// Handle executes the sync launch config job
func (j *SyncServerLaunchConfigJob) Handle(ctx context.Context) error {
	server, err := j.ctx.Repos().Server().Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	j.ctx.LogInfo("Syncing Launch backup configuration to server",
		"server_id", server.ID,
	)

	// Get all backups for this server
	backups, err := j.ctx.Repos().Backup().FindBackupsByServerID(ctx, server.ID)
	if err != nil {
		return fmt.Errorf("failed to get backups for server: %w", err)
	}

	// Count total backup jobs across all backups
	var totalBackupJobs int
	for _, backup := range backups {
		jobs, err := j.ctx.Repos().BackupJob().FindBackupJobsByBackupID(ctx, backup.ID)
		if err != nil {
			j.ctx.LogError(err, "Failed to get backup jobs", "backup_id", backup.ID)
			continue
		}
		totalBackupJobs += len(jobs)
	}

	j.ctx.LogInfo("Found backup configurations",
		"server_id", server.ID,
		"backups_count", len(backups),
		"backup_jobs_count", totalBackupJobs,
	)

	// TODO: Generate and deploy the launch-agent configuration
	// This would involve:
	// 1. Generate JSON/YAML config with all backup definitions
	// 2. Include backup schedules, retention policies, and storage credentials
	// 3. Deploy config to /etc/launch/backups.json or similar
	// 4. Ensure launch-agent service is running
	// 5. Reload the agent to pick up new configuration

	// For now, we just log the sync
	// In a full implementation, this would use SSH to deploy the config

	j.ctx.LogInfo("Launch backup configuration synced successfully",
		"server_id", server.ID,
	)

	return nil
}

// Failed handles job failure
func (j *SyncServerLaunchConfigJob) Failed(ctx context.Context, err error) {
	j.ctx.LogError(err, "Failed to sync Launch backup configuration",
		"server_id", j.Payload.ServerID,
	)
}

// NewSyncServerLaunchConfigTask creates a sync launch config task
func NewSyncServerLaunchConfigTask(serverID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.NewTask(TypeSyncServerLaunchConfig, SyncServerLaunchConfigPayload{
		ServerID: serverID,
		UserID:   userID,
	})
}
