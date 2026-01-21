package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/site/tasks"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeUninstallQueue = "site:uninstall_queue"

// UninstallQueuePayload holds data for queue worker uninstallation
type UninstallQueuePayload struct {
	SiteID  string  `json:"site_id"`
	QueueID string  `json:"queue_id"`
	UserID  *string `json:"user_id,omitempty"`
}

// UninstallQueueJob handles queue worker uninstallation
type UninstallQueueJob struct {
	pkgjobs.BaseJob[*JobContext, UninstallQueuePayload]
}

// NewUninstallQueueJob creates a new UninstallQueueJob with the given context and payload
func NewUninstallQueueJob(ctx *JobContext, payload UninstallQueuePayload) *UninstallQueueJob {
	return &UninstallQueueJob{
		BaseJob: pkgjobs.NewBaseJob(ctx, payload),
	}
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
	server, err := j.Ctx.ServerRepos.Server().FindByID(ctx, site.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	j.Ctx.LogInfo("Uninstalling queue worker",
		"queue_id", queue.ID,
		"site_id", site.ID,
	)

	// Delete the queue config and stop the process
	deleteTask := tasks.DeleteQueueConfig(queue.GetPath(), queue.ID)

	result, err := j.Ctx.RunTaskOnServer(server, deleteTask).AsRoot().Dispatch(ctx)
	if err != nil {
		j.Ctx.LogError(err, "Failed to delete queue config", "queue_id", queue.ID)
		return err
	}

	if result.GetExitCode() != 0 {
		j.Ctx.LogError(nil, "Queue deletion failed", "queue_id", queue.ID, "exit_code", result.GetExitCode())
	}

	// Delete the queue record from database
	if err := j.Ctx.QueueRepo.Delete(ctx, queue.ID); err != nil {
		j.Ctx.LogError(err, "Failed to delete queue record", "queue_id", queue.ID)
		return err
	}

	// Broadcast success
	j.Ctx.BroadcastServerEvent(server, "queue.uninstalled", map[string]interface{}{
		"site_id":  site.ID,
		"queue_id": queue.ID,
	})

	j.Ctx.LogInfo("Queue worker uninstalled successfully", "queue_id", queue.ID)

	return nil
}

// Failed handles job failure
func (j *UninstallQueueJob) Failed(ctx context.Context, err error) {
	j.Ctx.LogError(err, "Uninstall queue job failed",
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

// NewUninstallQueueTask creates an uninstall queue job
func NewUninstallQueueTask(siteID, queueID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.NewTask(TypeUninstallQueue, UninstallQueuePayload{
		SiteID:  siteID,
		QueueID: queueID,
		UserID:  userID,
	})
}
