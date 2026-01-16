package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeDisableLaravelHorizon = "site:disable_laravel_horizon"

// DisableLaravelHorizonPayload holds data for disabling Laravel Horizon
type DisableLaravelHorizonPayload struct {
	SiteID   string  `json:"site_id"`
	ServerID string  `json:"server_id"`
	UserID   *string `json:"user_id,omitempty"`
}

// DisableLaravelHorizonJob disables Laravel Horizon for a site
type DisableLaravelHorizonJob struct {
	ctx     *JobContext
	Payload DisableLaravelHorizonPayload
}

// NewDisableLaravelHorizonJob creates a new DisableLaravelHorizonJob
func NewDisableLaravelHorizonJob(ctx *JobContext, payload DisableLaravelHorizonPayload) *DisableLaravelHorizonJob {
	return &DisableLaravelHorizonJob{
		ctx:     ctx,
		Payload: payload,
	}
}

// Handle executes the disable Horizon job
func (j *DisableLaravelHorizonJob) Handle(ctx context.Context) error {
	// Get site
	site, err := j.ctx.SiteRepo.FindByID(ctx, j.Payload.SiteID)
	if err != nil {
		return fmt.Errorf("failed to find site: %w", err)
	}

	// Check if Horizon is enabled
	feature := site.GetEnabledFeature("horizon")
	if feature == nil || feature.QueueID == nil {
		j.ctx.LogInfo("Horizon not enabled, nothing to disable", "site_id", site.ID)
		return nil
	}

	queueID := *feature.QueueID

	// Get server
	server, err := j.ctx.ServerRepos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	j.ctx.LogInfo("Disabling Laravel Horizon",
		"site_id", site.ID,
		"server_id", server.ID,
		"queue_id", queueID,
	)

	// Dispatch UninstallQueue job
	if err := j.dispatchUninstallQueue(queueID, site.ID); err != nil {
		j.ctx.LogError(err, "Failed to dispatch uninstall queue job")
		// Continue to update the site's features even if dispatch fails
	}

	// Delete the queue record
	if err := j.ctx.QueueRepo.Delete(ctx, queueID); err != nil {
		j.ctx.LogError(err, "Failed to delete queue record", "queue_id", queueID)
	}

	// Update site's enabled_features
	site.RemoveEnabledFeature("horizon")
	site.RemovePendingFeature("horizon")

	if err := j.ctx.SiteRepo.UpdateFields(ctx, site.ID, map[string]interface{}{
		"enabled_features": site.EnabledFeatures,
		"pending_features": site.PendingFeatures,
	}); err != nil {
		j.ctx.LogError(err, "Failed to update site enabled_features")
	}

	// Broadcast success
	j.ctx.BroadcastServerEvent(server, "site.horizon_disabled", map[string]interface{}{
		"site_id": site.ID,
	})

	j.ctx.LogInfo("Laravel Horizon disabled successfully",
		"site_id", site.ID,
	)

	return nil
}

// dispatchUninstallQueue dispatches the UninstallQueue job
func (j *DisableLaravelHorizonJob) dispatchUninstallQueue(queueID, siteID string) error {
	if j.ctx.Queue == nil {
		return fmt.Errorf("queue client not available")
	}

	task, err := NewUninstallQueueTask(siteID, queueID, j.Payload.UserID)
	if err != nil {
		return err
	}

	_, err = j.ctx.Queue.Enqueue(task)
	return err
}

// Failed handles job failure
func (j *DisableLaravelHorizonJob) Failed(ctx context.Context, err error) {
	j.ctx.LogError(err, "Failed to disable Laravel Horizon",
		"site_id", j.Payload.SiteID,
	)

	// Remove from pending features
	site, findErr := j.ctx.SiteRepo.FindByID(ctx, j.Payload.SiteID)
	if findErr == nil {
		site.RemovePendingFeature("horizon")
		_ = j.ctx.SiteRepo.UpdateFields(ctx, site.ID, map[string]interface{}{
			"pending_features": site.PendingFeatures,
		})
	}
}

// NewDisableLaravelHorizonTask creates a disable Horizon task
func NewDisableLaravelHorizonTask(siteID, serverID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.NewTask(TypeDisableLaravelHorizon, DisableLaravelHorizonPayload{
		SiteID:   siteID,
		ServerID: serverID,
		UserID:   userID,
	})
}
