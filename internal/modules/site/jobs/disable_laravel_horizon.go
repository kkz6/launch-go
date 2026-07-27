package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/site/models"
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
	Deps    *JobDeps
	Payload DisableLaravelHorizonPayload

	// Model fields for Failed() callback
	site   *models.Site
	server *servermodels.Server
}

// NewDisableLaravelHorizonJob creates a new DisableLaravelHorizonJob
func NewDisableLaravelHorizonJob(p DisableLaravelHorizonPayload) pkgjobs.Handler {
	return &DisableLaravelHorizonJob{Deps: deps, Payload: p}
}

// Handle executes the disable Horizon job
func (j *DisableLaravelHorizonJob) Handle(ctx context.Context) error {
	// Get site
	site, err := j.Deps.Repos.Site().FindByID(ctx, j.Payload.SiteID)
	if err != nil {
		return fmt.Errorf("failed to find site: %w", err)
	}
	j.site = site

	// Check if Horizon is enabled
	feature := site.GetEnabledFeature("horizon")
	if feature == nil || feature.QueueID == nil {
		j.Deps.Logger.Info().Str("site_id", site.ID).Msg("Horizon not enabled, nothing to disable")
		return nil
	}

	queueID := *feature.QueueID

	// Get server
	server, err := j.Deps.ServerRepos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}
	j.server = server

	j.Deps.Logger.Info().
		Str("site_id", site.ID).
		Str("server_id", server.ID).
		Str("queue_id", queueID).
		Msg("Disabling Laravel Horizon")

	// Dispatch UninstallQueue job
	if err := j.dispatchUninstallQueue(queueID, site.ID); err != nil {
		j.Deps.Logger.Error().Err(err).Msg("Failed to dispatch uninstall queue job")
		// Continue to update the site's features even if dispatch fails
	}

	// Delete the queue record
	if err := j.Deps.Repos.Queue().Delete(ctx, queueID); err != nil {
		j.Deps.Logger.Error().Err(err).Str("queue_id", queueID).Msg("Failed to delete queue record")
	}

	// Update site's enabled_features
	site.RemoveEnabledFeature("horizon")
	site.RemovePendingFeature("horizon")

	if err := j.Deps.Repos.Site().UpdateFields(ctx, site.ID, map[string]interface{}{
		"enabled_features": site.EnabledFeatures,
		"pending_features": site.PendingFeatures,
	}); err != nil {
		j.Deps.Logger.Error().Err(err).Msg("Failed to update site enabled_features")
	}

	// Broadcast success
	j.Deps.BroadcastServerEvent(server, "site.horizon_disabled", map[string]interface{}{
		"site_id": site.ID,
	})

	j.Deps.Logger.Info().Str("site_id", site.ID).Msg("Laravel Horizon disabled successfully")

	return nil
}

// dispatchUninstallQueue dispatches the UninstallQueue job
func (j *DisableLaravelHorizonJob) dispatchUninstallQueue(queueID, siteID string) error {
	if j.Deps.Queue == nil {
		return fmt.Errorf("queue client not available")
	}

	task, err := NewUninstallQueueTask(siteID, queueID, j.Payload.UserID)
	if err != nil {
		return err
	}
	return j.Deps.DispatchTask(task)
}

// Failed handles job failure
func (j *DisableLaravelHorizonJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).Str("site_id", j.Payload.SiteID).Msg("Failed to disable Laravel Horizon")

	// Remove from pending features
	site, findErr := j.Deps.Repos.Site().FindByID(ctx, j.Payload.SiteID)
	if findErr == nil {
		site.RemovePendingFeature("horizon")
		if updateErr := j.Deps.Repos.Site().UpdateFields(ctx, site.ID, map[string]interface{}{
			"pending_features": site.PendingFeatures,
		}); updateErr != nil {
			j.Deps.Logger.Error().Err(updateErr).Str("site_id", site.ID).Msg("Failed to clear pending Horizon feature")
		}
	}
}

// NewDisableLaravelHorizonTask creates a disable Horizon task
func NewDisableLaravelHorizonTask(siteID, serverID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.Task(TypeDisableLaravelHorizon, DisableLaravelHorizonPayload{
		SiteID:   siteID,
		ServerID: serverID,
		UserID:   userID,
	})
}
