package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/kkz6/launch-go/internal/modules/site/tasks"
)

// UninstallQueueJob handles queue worker uninstallation
type UninstallQueueJob struct {
	SiteJobBase
	Payload UninstallQueuePayload
}

// Type returns the job type
func (j *UninstallQueueJob) Type() string {
	return TypeUninstallQueue
}

// Handle executes the uninstall queue job
func (j *UninstallQueueJob) Handle(ctx context.Context) error {
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

	j.LogInfo("Uninstalling queue worker",
		"queue_id", queue.ID,
		"site_id", site.ID,
	)

	// Delete the queue config and stop the process
	deleteTask := tasks.DeleteQueueConfig(queue.GetPath(), queue.ID)

	result, err := j.RunTaskOnServer(server, deleteTask).AsRoot().Dispatch(ctx)
	if err != nil {
		j.LogError(err, "Failed to delete queue config", "queue_id", queue.ID)
		return err
	}

	if result.GetExitCode() != 0 {
		j.LogError(nil, "Queue deletion failed", "queue_id", queue.ID, "exit_code", result.GetExitCode())
	}

	// Delete the queue record from database
	if err := j.Ctx.QueueRepo.Delete(ctx, queue.ID); err != nil {
		j.LogError(err, "Failed to delete queue record", "queue_id", queue.ID)
		return err
	}

	// Broadcast success
	j.BroadcastSiteEvent(site.ID, "queue.uninstalled", map[string]interface{}{
		"site_id":  site.ID,
		"queue_id": queue.ID,
	})

	j.LogInfo("Queue worker uninstalled successfully", "queue_id", queue.ID)

	return nil
}

// Failed handles job failure
func (j *UninstallQueueJob) Failed(ctx context.Context, err error) {
	j.LogError(err, "Uninstall queue job failed",
		"site_id", j.Payload.SiteID,
		"queue_id", j.Payload.QueueID,
	)

	// Mark uninstallation as failed
	queue, findErr := j.Ctx.QueueRepo.FindByID(ctx, j.Payload.QueueID)
	if findErr != nil {
		return
	}

	now := time.Now()
	queue.UninstallationFailedAt = &now
	_ = j.Ctx.QueueRepo.Update(ctx, queue)
}
