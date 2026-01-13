package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/pkg/jobs"
)

// UninstallCronJob uninstalls a cron job from a server.
// Similar to Laravel's Modules\Server\Jobs\UninstallCron
type UninstallCronJob struct {
	ServerJobBase
	jobs.UninstallationTracker
	Payload UninstallCronPayload
}

// Type returns the job type identifier
func (j *UninstallCronJob) Type() string {
	return TypeUninstallCron
}

// Handle processes the job
func (j *UninstallCronJob) Handle(ctx context.Context) error {
	// Find the cron with server preloaded
	cron, err := j.Repo().FindCronByIDWithServer(ctx, j.Payload.CronID)
	if err != nil {
		return fmt.Errorf("failed to find cron: %w", err)
	}

	// Delete cron file from server
	task := tasks.DeleteFile(tasks.DeleteFileConfig{
		Path: cron.Path(),
	})

	result, err := j.RunTaskOnServer(cron.Server, task).
		AsRoot().
		Dispatch(ctx)

	if err != nil {
		return fmt.Errorf("failed to delete cron file: %w", err)
	}

	if !result.IsSuccessful() {
		return fmt.Errorf("failed to delete cron file: %s", result.GetOutput())
	}

	// Delete the cron record
	if err := j.Repo().DeleteCron(ctx, cron.ID); err != nil {
		return fmt.Errorf("failed to delete cron: %w", err)
	}

	j.LogInfo("Cron uninstalled successfully",
		"cron_id", cron.ID,
		"server_id", cron.ServerID,
	)

	// Broadcast event
	j.BroadcastServerEvent(cron.ServerID, "cron.uninstalled", map[string]any{
		"cron_id":   cron.ID,
		"server_id": cron.ServerID,
	})

	return nil
}

// Failed is called when the job fails after all retries
func (j *UninstallCronJob) Failed(ctx context.Context, err error) {
	j.LogError(err, "Failed to uninstall cron",
		"cron_id", j.Payload.CronID,
		"server_id", j.Payload.ServerID,
	)

	// Mark uninstallation as failed
	cron, findErr := j.Repo().FindCronByID(ctx, j.Payload.CronID)
	if findErr == nil && cron != nil {
		j.MarkUninstallationFailed(j.DB, cron)
	}
}

// NewUninstallCronTask creates an asynq task for uninstalling a cron
func NewUninstallCronTask(serverID, cronID string, userID *string) (*asynq.Task, error) {
	return jobs.NewTask(TypeUninstallCron, UninstallCronPayload{
		ServerID: serverID,
		CronID:   cronID,
		UserID:   userID,
	})
}
