package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	serverjobs "github.com/kkz6/launch-go/internal/modules/server/jobs"
	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/site/models"
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
	Deps    *JobDeps
	Payload DisableLaravelSchedulerPayload

	// Model fields for Failed() callback
	site   *models.Site
	server *servermodels.Server
}

// NewDisableLaravelSchedulerJob creates a new DisableLaravelSchedulerJob
func NewDisableLaravelSchedulerJob(p DisableLaravelSchedulerPayload) pkgjobs.Handler {
	return &DisableLaravelSchedulerJob{Deps: deps, Payload: p}
}

// Handle executes the disable scheduler job
func (j *DisableLaravelSchedulerJob) Handle(ctx context.Context) error {
	// Get site
	site, err := j.Deps.Repos.Site().FindByID(ctx, j.Payload.SiteID)
	if err != nil {
		return fmt.Errorf("failed to find site: %w", err)
	}
	j.site = site

	// Check if scheduler is enabled
	feature := site.GetEnabledFeature("scheduler")
	if feature == nil || feature.CronID == nil {
		j.Deps.Logger.Info().Str("site_id", site.ID).Msg("Scheduler not enabled, nothing to disable")
		return nil
	}

	cronID := *feature.CronID

	// Get server
	server, err := j.Deps.ServerRepos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}
	j.server = server

	j.Deps.Logger.Info().
		Str("site_id", site.ID).
		Str("server_id", server.ID).
		Str("cron_id", cronID).
		Msg("Disabling Laravel scheduler")

	// Dispatch UninstallCron job
	if err := j.dispatchUninstallCron(cronID, server.ID); err != nil {
		j.Deps.Logger.Error().Err(err).Msg("Failed to dispatch uninstall cron job")
		// Continue to update the site's features even if dispatch fails
	}

	// Delete the cron record
	if err := j.Deps.ServerRepos.Cron().Delete(ctx, cronID); err != nil {
		j.Deps.Logger.Error().Err(err).Str("cron_id", cronID).Msg("Failed to delete cron record")
	}

	// Update site's enabled_features
	site.RemoveEnabledFeature("scheduler")
	site.RemovePendingFeature("scheduler")

	if err := j.Deps.Repos.Site().UpdateFields(ctx, site.ID, map[string]interface{}{
		"enabled_features": site.EnabledFeatures,
		"pending_features": site.PendingFeatures,
	}); err != nil {
		j.Deps.Logger.Error().Err(err).Msg("Failed to update site enabled_features")
	}

	// Broadcast success
	j.Deps.BroadcastServerEvent(server, "site.scheduler_disabled", map[string]interface{}{
		"site_id": site.ID,
	})

	j.Deps.Logger.Info().Str("site_id", site.ID).Msg("Laravel scheduler disabled successfully")

	return nil
}

// dispatchUninstallCron dispatches the server UninstallCron job
func (j *DisableLaravelSchedulerJob) dispatchUninstallCron(cronID, serverID string) error {
	task, err := serverjobs.NewUninstallCronTask(serverID, cronID, j.Payload.UserID)
	if err != nil {
		return err
	}
	return j.Deps.DispatchTask(task)
}

// Failed handles job failure
func (j *DisableLaravelSchedulerJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).Str("site_id", j.Payload.SiteID).Msg("Failed to disable Laravel scheduler")

	// Remove from pending features
	site, findErr := j.Deps.Repos.Site().FindByID(ctx, j.Payload.SiteID)
	if findErr == nil {
		site.RemovePendingFeature("scheduler")
		if updateErr := j.Deps.Repos.Site().UpdateFields(ctx, site.ID, map[string]interface{}{
			"pending_features": site.PendingFeatures,
		}); updateErr != nil {
			j.Deps.Logger.Error().Err(updateErr).Str("site_id", site.ID).Msg("Failed to clear pending scheduler feature")
		}
	}
}

// NewDisableLaravelSchedulerTask creates a disable scheduler task
func NewDisableLaravelSchedulerTask(siteID, serverID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.Task(TypeDisableLaravelScheduler, DisableLaravelSchedulerPayload{
		SiteID:   siteID,
		ServerID: serverID,
		UserID:   userID,
	})
}
