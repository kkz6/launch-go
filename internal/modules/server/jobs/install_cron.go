package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/pkg/jobs"
)

// InstallCronJob installs a cron job on a server.
// Similar to Laravel's Modules\Server\Jobs\InstallCron
type InstallCronJob struct {
	ServerJobBase
	jobs.InstallationTracker
	Payload InstallCronPayload
}

// Type returns the job type identifier
func (j *InstallCronJob) Type() string {
	return TypeInstallCron
}

// Handle processes the job
func (j *InstallCronJob) Handle(ctx context.Context) error {
	// Find the cron with server preloaded (using repository)
	cron, err := j.Repo().FindCronByIDWithServer(ctx, j.Payload.CronID)
	if err != nil {
		return fmt.Errorf("failed to find cron: %w", err)
	}

	// Build cron file contents
	contents := cron.ToCronFileContents()

	// Create and run the upload task
	// This follows the Laravel pattern: $server->runTask(task)->asRoot()->dispatch()
	task := tasks.UploadCron(tasks.UploadCronConfig{
		Path:     cron.Path(),
		Contents: contents,
	})

	result, err := j.RunTaskOnServer(cron.Server, task).
		AsRoot().
		Dispatch(ctx)

	if err != nil {
		return fmt.Errorf("failed to upload cron file: %w", err)
	}

	if !result.IsSuccessful() {
		return fmt.Errorf("failed to upload cron file: %s", result.GetOutput())
	}

	// Mark as installed (using repository)
	if err := j.Repo().MarkCronInstalled(ctx, cron.ID); err != nil {
		return fmt.Errorf("failed to mark cron as installed: %w", err)
	}

	j.LogInfo("Cron installed successfully",
		"cron_id", cron.ID,
		"server_id", cron.ServerID,
		"command", cron.Command.String(),
	)

	// Broadcast event
	j.BroadcastServerEvent(cron.ServerID, "cron.installed", map[string]interface{}{
		"cron_id":   cron.ID,
		"server_id": cron.ServerID,
	})

	return nil
}

// Failed is called when the job fails after all retries
func (j *InstallCronJob) Failed(ctx context.Context, err error) {
	j.LogError(err, "Failed to install cron",
		"cron_id", j.Payload.CronID,
		"server_id", j.Payload.ServerID,
	)

	// Mark installation as failed
	cron, findErr := j.Repo().FindCronByID(ctx, j.Payload.CronID)
	if findErr == nil && cron != nil {
		j.MarkInstallationFailed(j.DB, cron)
	}
}

// NewInstallCronTask creates an asynq task for installing a cron
func NewInstallCronTask(serverID, cronID string, userID *string) (*asynq.Task, error) {
	return jobs.NewTask(TypeInstallCron, InstallCronPayload{
		ServerID: serverID,
		CronID:   cronID,
		UserID:   userID,
	})
}
