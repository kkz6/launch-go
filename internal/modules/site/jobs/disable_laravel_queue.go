package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeDisableLaravelQueue = "site:disable_laravel_queue"

// DisableLaravelQueuePayload holds data for disabling Laravel queue
type DisableLaravelQueuePayload struct {
	SiteID   string  `json:"site_id"`
	ServerID string  `json:"server_id"`
	UserID   *string `json:"user_id,omitempty"`
}

// DisableLaravelQueueJob disables the Laravel queue worker for a site
type DisableLaravelQueueJob struct {
	pkgjobs.BaseJob[*JobContext, DisableLaravelQueuePayload]
}

// NewDisableLaravelQueueJob creates a new DisableLaravelQueueJob
func NewDisableLaravelQueueJob(ctx *JobContext, payload DisableLaravelQueuePayload) *DisableLaravelQueueJob {
	return &DisableLaravelQueueJob{
		BaseJob: pkgjobs.NewBaseJob(ctx, payload),
	}
}

// Handle executes the disable queue job
func (j *DisableLaravelQueueJob) Handle(ctx context.Context) error {
	// Get site
	site, err := j.Ctx.SiteRepo.FindByID(ctx, j.Payload.SiteID)
	if err != nil {
		return fmt.Errorf("failed to find site: %w", err)
	}

	// Check if queue is enabled
	feature := site.GetEnabledFeature("queue")
	if feature == nil || feature.QueueID == nil {
		j.Ctx.LogInfo("Queue not enabled, nothing to disable", "site_id", site.ID)
		return nil
	}

	queueID := *feature.QueueID

	// Get server
	server, err := j.Ctx.ServerRepos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	j.Ctx.LogInfo("Disabling Laravel queue worker",
		"site_id", site.ID,
		"server_id", server.ID,
		"queue_id", queueID,
	)

	// Dispatch UninstallQueue job
	if err := j.dispatchUninstallQueue(queueID, site.ID); err != nil {
		j.Ctx.LogError(err, "Failed to dispatch uninstall queue job")
		// Continue to update the site's features even if dispatch fails
	}

	// Delete the queue record
	if err := j.Ctx.QueueRepo.Delete(ctx, queueID); err != nil {
		j.Ctx.LogError(err, "Failed to delete queue record", "queue_id", queueID)
	}

	// Update site's enabled_features
	site.RemoveEnabledFeature("queue")
	site.RemovePendingFeature("queue")

	if err := j.Ctx.SiteRepo.UpdateFields(ctx, site.ID, map[string]interface{}{
		"enabled_features": site.EnabledFeatures,
		"pending_features": site.PendingFeatures,
	}); err != nil {
		j.Ctx.LogError(err, "Failed to update site enabled_features")
	}

	// Broadcast success
	j.Ctx.BroadcastServerEvent(server, "site.queue_disabled", map[string]interface{}{
		"site_id": site.ID,
	})

	j.Ctx.LogInfo("Laravel queue worker disabled successfully",
		"site_id", site.ID,
	)

	return nil
}

// dispatchUninstallQueue dispatches the UninstallQueue job
func (j *DisableLaravelQueueJob) dispatchUninstallQueue(queueID, siteID string) error {
	task, err := NewUninstallQueueTask(siteID, queueID, j.Payload.UserID)
	if err != nil {
		return err
	}
	return j.Ctx.DispatchTask(task)
}

// Failed handles job failure
func (j *DisableLaravelQueueJob) Failed(ctx context.Context, err error) {
	j.Ctx.LogError(err, "Failed to disable Laravel queue worker",
		"site_id", j.Payload.SiteID,
	)

	// Remove from pending features
	site, findErr := j.Ctx.SiteRepo.FindByID(ctx, j.Payload.SiteID)
	if findErr == nil {
		site.RemovePendingFeature("queue")
		_ = j.Ctx.SiteRepo.UpdateFields(ctx, site.ID, map[string]interface{}{
			"pending_features": site.PendingFeatures,
		})
	}
}

// NewDisableLaravelQueueTask creates a disable queue task
func NewDisableLaravelQueueTask(siteID, serverID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.NewTask(TypeDisableLaravelQueue, DisableLaravelQueuePayload{
		SiteID:   siteID,
		ServerID: serverID,
		UserID:   userID,
	})
}
