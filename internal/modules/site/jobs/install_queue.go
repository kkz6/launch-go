package jobs

import (
	"context"
	"fmt"
	"time"

	servertasks "github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/modules/site/tasks"
)

// InstallQueueJob handles queue worker installation
type InstallQueueJob struct {
	SiteJobBase
	Payload InstallQueuePayload
}

// Type returns the job type
func (j *InstallQueueJob) Type() string {
	return TypeInstallQueue
}

// Handle executes the install queue job
func (j *InstallQueueJob) Handle(ctx context.Context) error {
	// Get queue
	queue, err := j.Ctx.QueueRepo.FindByID(ctx, j.Payload.QueueID)
	if err != nil {
		return fmt.Errorf("failed to find queue: %w", err)
	}

	// Get site
	site, err := j.Ctx.SiteRepo.FindByID(ctx, j.Payload.SiteID)
	if err != nil {
		return fmt.Errorf("failed to find site: %w", err)
	}

	// Get server
	server, err := j.Ctx.ServerRepo.FindServerByID(ctx, site.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	j.LogInfo("Installing queue worker",
		"queue_id", queue.ID,
		"site_id", site.ID,
		"command", queue.Command,
	)

	// Get server username
	serverUsername := "launch"
	if server.Username != nil {
		serverUsername = *server.Username
	}

	// Build supervisor config
	configContent := tasks.BuildQueueSupervisorConfig(queue, serverUsername)
	configPath := queue.GetPath()

	// Upload the config
	uploadTask := tasks.UploadQueueConfig(tasks.UploadQueueConfigParams{
		Path:         configPath,
		Contents:     configContent,
		LogPath:      queue.GetLogPath(),
		ErrorLogPath: queue.GetErrorLogPath(),
		User:         queue.User,
	})

	result, err := j.RunTaskOnServer(server, uploadTask).AsRoot().Dispatch(ctx)
	if err != nil {
		j.LogError(err, "Failed to upload queue config", "queue_id", queue.ID)
		return err
	}

	if result.GetExitCode() != 0 {
		return fmt.Errorf("failed to upload queue config: exit code %d", result.GetExitCode())
	}

	// Reload supervisor
	reloadTask := servertasks.ReloadSupervisor()
	result, err = j.RunTaskOnServer(server, reloadTask).AsRoot().Dispatch(ctx)
	if err != nil {
		j.LogError(err, "Failed to reload supervisor", "queue_id", queue.ID)
		return err
	}

	if result.GetExitCode() != 0 {
		j.LogError(nil, "Supervisor reload failed", "queue_id", queue.ID, "exit_code", result.GetExitCode())
	}

	// Mark queue as installed
	now := time.Now()
	queue.InstalledAt = &now
	queue.InstallationFailedAt = nil
	if err := j.Ctx.QueueRepo.Update(ctx, queue); err != nil {
		j.LogError(err, "Failed to update queue installed status")
	}

	// Broadcast success
	j.BroadcastSiteEvent(site.ID, "queue.installed", map[string]interface{}{
		"site_id":  site.ID,
		"queue_id": queue.ID,
	})

	j.LogInfo("Queue worker installed successfully", "queue_id", queue.ID)

	return nil
}

// Failed handles job failure
func (j *InstallQueueJob) Failed(ctx context.Context, err error) {
	j.LogError(err, "Install queue job failed",
		"site_id", j.Payload.SiteID,
		"queue_id", j.Payload.QueueID,
	)

	// Mark installation as failed
	queue, findErr := j.Ctx.QueueRepo.FindByID(ctx, j.Payload.QueueID)
	if findErr != nil {
		return
	}

	now := time.Now()
	queue.InstalledAt = nil
	queue.InstallationFailedAt = &now
	_ = j.Ctx.QueueRepo.Update(ctx, queue)
}
