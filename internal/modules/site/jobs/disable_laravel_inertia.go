package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeDisableLaravelInertia = "site:disable_laravel_inertia"

// DisableLaravelInertiaPayload holds data for disabling Laravel Inertia SSR
type DisableLaravelInertiaPayload struct {
	SiteID   string  `json:"site_id"`
	ServerID string  `json:"server_id"`
	UserID   *string `json:"user_id,omitempty"`
}

// DisableLaravelInertiaJob disables Inertia SSR for a Laravel site
type DisableLaravelInertiaJob struct {
	pkgjobs.BaseJob[*JobContext, DisableLaravelInertiaPayload]
}

// NewDisableLaravelInertiaJob creates a new DisableLaravelInertiaJob
func NewDisableLaravelInertiaJob(ctx *JobContext, payload DisableLaravelInertiaPayload) *DisableLaravelInertiaJob {
	return &DisableLaravelInertiaJob{
		BaseJob: pkgjobs.NewBaseJob(ctx, payload),
	}
}

// Handle executes the disable Inertia job
func (j *DisableLaravelInertiaJob) Handle(ctx context.Context) error {
	// Get site
	site, err := j.Ctx.SiteRepo.FindByID(ctx, j.Payload.SiteID)
	if err != nil {
		return fmt.Errorf("failed to find site: %w", err)
	}

	// Check if Inertia is enabled
	feature := site.GetEnabledFeature("inertia")
	if feature == nil || feature.QueueID == nil {
		j.Ctx.LogInfo("Inertia SSR not enabled, nothing to disable", "site_id", site.ID)
		return nil
	}

	queueID := *feature.QueueID

	// Get server
	server, err := j.Ctx.ServerRepos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	j.Ctx.LogInfo("Disabling Laravel Inertia SSR",
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
	site.RemoveEnabledFeature("inertia")
	site.RemovePendingFeature("inertia")

	if err := j.Ctx.SiteRepo.UpdateFields(ctx, site.ID, map[string]interface{}{
		"enabled_features": site.EnabledFeatures,
		"pending_features": site.PendingFeatures,
	}); err != nil {
		j.Ctx.LogError(err, "Failed to update site enabled_features")
	}

	// Broadcast success
	j.Ctx.BroadcastServerEvent(server, "site.inertia_disabled", map[string]interface{}{
		"site_id": site.ID,
	})

	j.Ctx.LogInfo("Laravel Inertia SSR disabled successfully",
		"site_id", site.ID,
	)

	return nil
}

// dispatchUninstallQueue dispatches the UninstallQueue job
func (j *DisableLaravelInertiaJob) dispatchUninstallQueue(queueID, siteID string) error {
	if j.Ctx.Queue == nil {
		return fmt.Errorf("queue client not available")
	}

	task, err := NewUninstallQueueTask(siteID, queueID, j.Payload.UserID)
	if err != nil {
		return err
	}

	_, err = j.Ctx.Queue.Enqueue(task)
	return err
}

// Failed handles job failure
func (j *DisableLaravelInertiaJob) Failed(ctx context.Context, err error) {
	j.Ctx.LogError(err, "Failed to disable Laravel Inertia SSR",
		"site_id", j.Payload.SiteID,
	)

	// Remove from pending features
	site, findErr := j.Ctx.SiteRepo.FindByID(ctx, j.Payload.SiteID)
	if findErr == nil {
		site.RemovePendingFeature("inertia")
		_ = j.Ctx.SiteRepo.UpdateFields(ctx, site.ID, map[string]interface{}{
			"pending_features": site.PendingFeatures,
		})
	}
}

// NewDisableLaravelInertiaTask creates a disable Inertia task
func NewDisableLaravelInertiaTask(siteID, serverID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.NewTask(TypeDisableLaravelInertia, DisableLaravelInertiaPayload{
		SiteID:   siteID,
		ServerID: serverID,
		UserID:   userID,
	})
}
