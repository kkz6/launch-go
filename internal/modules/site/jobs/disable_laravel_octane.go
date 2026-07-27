package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeDisableLaravelOctane = "site:disable_laravel_octane"

// DisableLaravelOctanePayload holds data for disabling Laravel Octane
type DisableLaravelOctanePayload struct {
	SiteID   string  `json:"site_id"`
	ServerID string  `json:"server_id"`
	UserID   *string `json:"user_id,omitempty"`
}

// DisableLaravelOctaneJob disables Laravel Octane for a site
type DisableLaravelOctaneJob struct {
	Deps    *JobDeps
	Payload DisableLaravelOctanePayload

	// Model fields for Failed() callback
	site   *models.Site
	server *servermodels.Server
}

// NewDisableLaravelOctaneJob creates a new DisableLaravelOctaneJob
func NewDisableLaravelOctaneJob(p DisableLaravelOctanePayload) pkgjobs.Handler {
	return &DisableLaravelOctaneJob{Deps: deps, Payload: p}
}

// Handle executes the disable Octane job
func (j *DisableLaravelOctaneJob) Handle(ctx context.Context) error {
	site, err := j.Deps.Repos.Site().FindByID(ctx, j.Payload.SiteID)
	if err != nil {
		return fmt.Errorf("failed to find site: %w", err)
	}
	j.site = site

	feature := site.GetEnabledFeature(FeatureOctane)
	if feature == nil || feature.QueueID == nil {
		j.Deps.Logger.Info().Str("site_id", site.ID).Msg("Octane not enabled, nothing to disable")
		return nil
	}

	queueID := *feature.QueueID

	server, err := j.Deps.ServerRepos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}
	j.server = server

	j.Deps.Logger.Info().
		Str("site_id", site.ID).
		Str("server_id", server.ID).
		Str("queue_id", queueID).
		Msg("Disabling Laravel Octane")

	// Dispatch UninstallQueue job
	if err := j.dispatchUninstallQueue(queueID, site.ID); err != nil {
		j.Deps.Logger.Error().Err(err).Msg("Failed to dispatch uninstall queue job")
	}

	// Delete the queue record
	if err := j.Deps.Repos.Queue().Delete(ctx, queueID); err != nil {
		j.Deps.Logger.Error().Err(err).Str("queue_id", queueID).Msg("Failed to delete queue record")
	}

	// Update site's enabled_features
	site.RemoveEnabledFeature(FeatureOctane)
	site.RemovePendingFeature(FeatureOctane)

	if err := j.Deps.Repos.Site().UpdateFields(ctx, site.ID, map[string]interface{}{
		"enabled_features": site.EnabledFeatures,
		"pending_features": site.PendingFeatures,
	}); err != nil {
		j.Deps.Logger.Error().Err(err).Msg("Failed to update site enabled_features")
	}

	// Update Caddyfile to revert to php_fastcgi
	if err := j.dispatchUpdateCaddyfile(site.ID); err != nil {
		j.Deps.Logger.Error().Err(err).Msg("Failed to dispatch update Caddyfile job")
	}

	j.Deps.BroadcastServerEvent(server, "site.octane_disabled", map[string]interface{}{
		"site_id": site.ID,
	})

	j.Deps.Logger.Info().Str("site_id", site.ID).Msg("Laravel Octane disabled successfully")

	return nil
}

// dispatchUninstallQueue dispatches the UninstallQueue job
func (j *DisableLaravelOctaneJob) dispatchUninstallQueue(queueID, siteID string) error {
	if j.Deps.Queue == nil {
		return fmt.Errorf("queue client not available")
	}

	task, err := NewUninstallQueueTask(siteID, queueID, j.Payload.UserID)
	if err != nil {
		return err
	}
	return j.Deps.DispatchTask(task)
}

// dispatchUpdateCaddyfile dispatches the UpdateCaddyfile job
func (j *DisableLaravelOctaneJob) dispatchUpdateCaddyfile(siteID string) error {
	task, err := NewUpdateCaddyfileTask(siteID, j.Payload.UserID)
	if err != nil {
		return err
	}
	return j.Deps.DispatchTask(task)
}

// Failed handles job failure
func (j *DisableLaravelOctaneJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).Str("site_id", j.Payload.SiteID).Msg("Failed to disable Laravel Octane")

	site, findErr := j.Deps.Repos.Site().FindByID(ctx, j.Payload.SiteID)
	if findErr == nil {
		site.RemovePendingFeature(FeatureOctane)
		if updateErr := j.Deps.Repos.Site().UpdateFields(ctx, site.ID, map[string]interface{}{
			"pending_features": site.PendingFeatures,
		}); updateErr != nil {
			j.Deps.Logger.Error().Err(updateErr).Str("site_id", site.ID).Msg("Failed to clear pending Octane feature")
		}
	}
}

// NewDisableLaravelOctaneTask creates a disable Octane task
func NewDisableLaravelOctaneTask(siteID, serverID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.Task(TypeDisableLaravelOctane, DisableLaravelOctanePayload{
		SiteID:   siteID,
		ServerID: serverID,
		UserID:   userID,
	})
}
