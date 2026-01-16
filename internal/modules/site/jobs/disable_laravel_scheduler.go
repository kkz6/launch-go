package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	serverjobs "github.com/kkz6/launch-go/internal/modules/server/jobs"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeDisableLaravelScheduler = "site:disable_laravel_scheduler"

// DisableLaravelSchedulerPayload holds data for disabling Laravel scheduler
type DisableLaravelSchedulerPayload struct {
	SiteID   string  `json:"site_id"`
	ServerID string  `json:"server_id"`
	UserID   *string `json:"user_id,omitempty"`
}

// DisableLaravelSchedulerJob disables the Laravel scheduler cron for a site
type DisableLaravelSchedulerJob struct {
	ctx     *JobContext
	Payload DisableLaravelSchedulerPayload
}

// NewDisableLaravelSchedulerJob creates a new DisableLaravelSchedulerJob
func NewDisableLaravelSchedulerJob(ctx *JobContext, payload DisableLaravelSchedulerPayload) *DisableLaravelSchedulerJob {
	return &DisableLaravelSchedulerJob{
		ctx:     ctx,
		Payload: payload,
	}
}

// Handle executes the disable scheduler job
func (j *DisableLaravelSchedulerJob) Handle(ctx context.Context) error {
	// Get site
	site, err := j.ctx.SiteRepo.FindByID(ctx, j.Payload.SiteID)
	if err != nil {
		return fmt.Errorf("failed to find site: %w", err)
	}

	// Check if scheduler is enabled
	feature := site.GetEnabledFeature("scheduler")
	if feature == nil || feature.CronID == nil {
		j.ctx.LogInfo("Scheduler not enabled, nothing to disable", "site_id", site.ID)
		return nil
	}

	cronID := *feature.CronID

	// Get server
	server, err := j.ctx.ServerRepos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	j.ctx.LogInfo("Disabling Laravel scheduler",
		"site_id", site.ID,
		"server_id", server.ID,
		"cron_id", cronID,
	)

	// Dispatch UninstallCron job
	if err := j.dispatchUninstallCron(cronID, server.ID); err != nil {
		j.ctx.LogError(err, "Failed to dispatch uninstall cron job")
		// Continue to update the site's features even if dispatch fails
	}

	// Delete the cron record
	if err := j.ctx.ServerRepos.Cron().Delete(ctx, cronID); err != nil {
		j.ctx.LogError(err, "Failed to delete cron record", "cron_id", cronID)
	}

	// Update site's enabled_features
	site.RemoveEnabledFeature("scheduler")
	site.RemovePendingFeature("scheduler")

	if err := j.ctx.SiteRepo.UpdateFields(ctx, site.ID, map[string]interface{}{
		"enabled_features": site.EnabledFeatures,
		"pending_features": site.PendingFeatures,
	}); err != nil {
		j.ctx.LogError(err, "Failed to update site enabled_features")
	}

	// Broadcast success
	j.ctx.BroadcastServerEvent(server, "site.scheduler_disabled", map[string]interface{}{
		"site_id": site.ID,
	})

	j.ctx.LogInfo("Laravel scheduler disabled successfully",
		"site_id", site.ID,
	)

	return nil
}

// dispatchUninstallCron dispatches the server UninstallCron job
func (j *DisableLaravelSchedulerJob) dispatchUninstallCron(cronID, serverID string) error {
	if j.ctx.Queue == nil {
		return fmt.Errorf("queue client not available")
	}

	task, err := serverjobs.NewUninstallCronTask(serverID, cronID, j.Payload.UserID)
	if err != nil {
		return err
	}

	_, err = j.ctx.Queue.Enqueue(task)
	return err
}

// Failed handles job failure
func (j *DisableLaravelSchedulerJob) Failed(ctx context.Context, err error) {
	j.ctx.LogError(err, "Failed to disable Laravel scheduler",
		"site_id", j.Payload.SiteID,
	)

	// Remove from pending features
	site, findErr := j.ctx.SiteRepo.FindByID(ctx, j.Payload.SiteID)
	if findErr == nil {
		site.RemovePendingFeature("scheduler")
		_ = j.ctx.SiteRepo.UpdateFields(ctx, site.ID, map[string]interface{}{
			"pending_features": site.PendingFeatures,
		})
	}
}

// NewDisableLaravelSchedulerTask creates a disable scheduler task
func NewDisableLaravelSchedulerTask(siteID, serverID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.NewTask(TypeDisableLaravelScheduler, DisableLaravelSchedulerPayload{
		SiteID:   siteID,
		ServerID: serverID,
		UserID:   userID,
	})
}
