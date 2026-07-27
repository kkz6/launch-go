package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/site/models"
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
	Deps    *JobDeps
	Payload DisableLaravelQueuePayload

	// Model fields for Failed() callback
	site   *models.Site
	server *servermodels.Server
}

// NewDisableLaravelQueueJob creates a new DisableLaravelQueueJob
func NewDisableLaravelQueueJob(p DisableLaravelQueuePayload) pkgjobs.Handler {
	return &DisableLaravelQueueJob{Deps: deps, Payload: p}
}

// Handle executes the disable queue job
func (j *DisableLaravelQueueJob) Handle(ctx context.Context) error {
	// Get site
	site, err := j.Deps.Repos.Site().FindByID(ctx, j.Payload.SiteID)
	if err != nil {
		return fmt.Errorf("failed to find site: %w", err)
	}
	j.site = site

	// Check if queue is enabled
	feature := site.GetEnabledFeature("queue")
	if feature == nil || feature.QueueID == nil {
		j.Deps.Logger.Info().Str("site_id", site.ID).Msg("Queue not enabled, nothing to disable")
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
		Msg("Disabling Laravel queue worker")

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
	site.RemoveEnabledFeature("queue")
	site.RemovePendingFeature("queue")

	if err := j.Deps.Repos.Site().UpdateFields(ctx, site.ID, map[string]interface{}{
		"enabled_features": site.EnabledFeatures,
		"pending_features": site.PendingFeatures,
	}); err != nil {
		j.Deps.Logger.Error().Err(err).Msg("Failed to update site enabled_features")
	}

	// Broadcast success
	j.Deps.BroadcastServerEvent(server, "site.queue_disabled", map[string]interface{}{
		"site_id": site.ID,
	})

	j.Deps.Logger.Info().Str("site_id", site.ID).Msg("Laravel queue worker disabled successfully")

	return nil
}

// dispatchUninstallQueue dispatches the UninstallQueue job
func (j *DisableLaravelQueueJob) dispatchUninstallQueue(queueID, siteID string) error {
	task, err := NewUninstallQueueTask(siteID, queueID, j.Payload.UserID)
	if err != nil {
		return err
	}
	return j.Deps.DispatchTask(task)
}

// Failed handles job failure
func (j *DisableLaravelQueueJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).Str("site_id", j.Payload.SiteID).Msg("Failed to disable Laravel queue worker")

	// Remove from pending features
	site, findErr := j.Deps.Repos.Site().FindByID(ctx, j.Payload.SiteID)
	if findErr == nil {
		site.RemovePendingFeature("queue")
		if updateErr := j.Deps.Repos.Site().UpdateFields(ctx, site.ID, map[string]interface{}{
			"pending_features": site.PendingFeatures,
		}); updateErr != nil {
			j.Deps.Logger.Error().Err(updateErr).Str("site_id", site.ID).Msg("Failed to clear pending Queue feature")
		}
	}
}

// NewDisableLaravelQueueTask creates a disable queue task
func NewDisableLaravelQueueTask(siteID, serverID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.Task(TypeDisableLaravelQueue, DisableLaravelQueuePayload{
		SiteID:   siteID,
		ServerID: serverID,
		UserID:   userID,
	})
}
