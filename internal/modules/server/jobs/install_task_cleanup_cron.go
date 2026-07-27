package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/types"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeInstallTaskCleanupCron = "server:install_task_cleanup_cron"

// InstallTaskCleanupCronPayload holds data for task cleanup cron installation
type InstallTaskCleanupCronPayload struct {
	ServerID string  `json:"server_id"`
	UserID   *string `json:"user_id,omitempty"`
}

// InstallTaskCleanupCronJob installs a cron job to cleanup old task records
type InstallTaskCleanupCronJob struct {
	Deps    *JobDeps
	Payload InstallTaskCleanupCronPayload

	server *models.Server
}

func NewInstallTaskCleanupCronJob(p InstallTaskCleanupCronPayload) pkgjobs.Handler {
	return &InstallTaskCleanupCronJob{Deps: deps, Payload: p}
}

// Handle executes the install task cleanup cron job
func (j *InstallTaskCleanupCronJob) Handle(ctx context.Context) error {
	var err error
	j.server, err = j.Deps.Repos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	j.Deps.Logger.Info().
		Str("server_id", j.server.ID).
		Msg("installing task cleanup cron")

	// Build cleanup command
	// This command cleans up old task output files and temporary files
	cleanupCommand := `find /tmp -name "launch_task_*" -mtime +7 -delete 2>/dev/null; find /var/log/launch -name "*.log" -mtime +30 -delete 2>/dev/null`

	// Create cron record (runs daily at 3 AM)
	schedule := types.CronDaily3AM
	cron := &models.Cron{
		Expression: schedule.Expression(),
		Command:    dbtype.EncryptedString(cleanupCommand),
		User:       "root",
		Frequency:  schedule.FrequencyName(),
		Hidden:     true, // System cron, hidden from user
	}
	cron.ServerID = j.server.ID

	if err := j.Deps.Repos.Cron().Create(ctx, cron); err != nil {
		return fmt.Errorf("failed to create cron: %w", err)
	}

	// Dispatch InstallCron job to install it on the server
	if err := j.dispatchInstallCron(cron.ID, j.server.ID); err != nil {
		// Cleanup the cron record if dispatch fails
		if cleanupErr := j.Deps.Repos.Cron().Delete(ctx, cron.ID); cleanupErr != nil {
			j.Deps.Logger.Error().Err(cleanupErr).Str("cron_id", cron.ID).Msg("Failed to clean up task cleanup cron after dispatch failure")
		}
		return fmt.Errorf("failed to dispatch install cron job: %w", err)
	}

	j.Deps.Logger.Info().
		Str("server_id", j.server.ID).
		Str("cron_id", cron.ID).
		Msg("task cleanup cron installed successfully")

	j.Deps.BroadcastServerEvent(j.server, "cron.task_cleanup_installed", map[string]any{
		"server_id": j.server.ID,
		"cron_id":   cron.ID,
	})

	return nil
}

// dispatchInstallCron dispatches the InstallCron job
func (j *InstallTaskCleanupCronJob) dispatchInstallCron(cronID, serverID string) error {
	task, err := NewInstallCronTask(serverID, cronID, j.Payload.UserID)
	if err != nil {
		return err
	}
	return j.Deps.DispatchTask(task)
}

// Failed handles job failure
func (j *InstallTaskCleanupCronJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).
		Str("server_id", j.Payload.ServerID).
		Msg("failed to install task cleanup cron")
}

// NewInstallTaskCleanupCronTask creates an install task cleanup cron task
func NewInstallTaskCleanupCronTask(serverID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.Task(TypeInstallTaskCleanupCron, InstallTaskCleanupCronPayload{
		ServerID: serverID,
		UserID:   userID,
	}, asynq.TaskID(pkgjobs.Dedup("install_task_cleanup_cron", serverID)))
}
