package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

const TypeInstallTaskCleanupCron = "server:install_task_cleanup_cron"

// InstallTaskCleanupCronPayload holds data for task cleanup cron installation
type InstallTaskCleanupCronPayload struct {
	ServerID string  `json:"server_id"`
	UserID   *string `json:"user_id,omitempty"`
}

// InstallTaskCleanupCronJob installs a cron job to cleanup old task records
type InstallTaskCleanupCronJob struct {
	pkgjobs.BaseJob[*JobContext, InstallTaskCleanupCronPayload]
}

// NewInstallTaskCleanupCronJob creates a new InstallTaskCleanupCronJob
func NewInstallTaskCleanupCronJob(ctx *JobContext, payload InstallTaskCleanupCronPayload) *InstallTaskCleanupCronJob {
	return &InstallTaskCleanupCronJob{
		BaseJob: pkgjobs.NewBaseJob(ctx, payload),
	}
}

// Handle executes the install task cleanup cron job
func (j *InstallTaskCleanupCronJob) Handle(ctx context.Context) error {
	server, err := j.Ctx.Repos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	j.Ctx.LogInfo("Installing task cleanup cron",
		"server_id", server.ID,
	)

	// Build cleanup command
	// This command cleans up old task output files and temporary files
	cleanupCommand := `find /tmp -name "launch_task_*" -mtime +7 -delete 2>/dev/null; find /var/log/launch -name "*.log" -mtime +30 -delete 2>/dev/null`

	// Create cron record (runs daily at 3 AM)
	cron := &models.Cron{
		ServerID:   server.ID,
		Expression: "0 3 * * *",
		Command:    basemodels.EncryptedString(cleanupCommand),
		User:       "root",
		Frequency:  "daily",
		Hidden:     true, // System cron, hidden from user
	}

	if err := j.Ctx.Repos.Cron().Create(ctx, cron); err != nil {
		return fmt.Errorf("failed to create cron: %w", err)
	}

	// Dispatch InstallCron job to install it on the server
	if err := j.dispatchInstallCron(cron.ID, server.ID); err != nil {
		// Cleanup the cron record if dispatch fails
		_ = j.Ctx.Repos.Cron().Delete(ctx, cron.ID)
		return fmt.Errorf("failed to dispatch install cron job: %w", err)
	}

	j.Ctx.LogInfo("Task cleanup cron installed successfully",
		"server_id", server.ID,
		"cron_id", cron.ID,
	)

	j.Ctx.BroadcastServerEvent(server, "cron.task_cleanup_installed", map[string]any{
		"server_id": server.ID,
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
	return j.Ctx.DispatchTask(task)
}

// Failed handles job failure
func (j *InstallTaskCleanupCronJob) Failed(ctx context.Context, err error) {
	j.Ctx.LogError(err, "Failed to install task cleanup cron",
		"server_id", j.Payload.ServerID,
	)
}

// NewInstallTaskCleanupCronTask creates an install task cleanup cron task
func NewInstallTaskCleanupCronTask(serverID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.NewTask(TypeInstallTaskCleanupCron, InstallTaskCleanupCronPayload{
		ServerID: serverID,
		UserID:   userID,
	})
}
