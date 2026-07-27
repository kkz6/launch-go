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

const TypeDisableLaravelReverb = "site:disable_laravel_reverb"

// DisableLaravelReverbPayload holds data for disabling Laravel Reverb
type DisableLaravelReverbPayload struct {
	SiteID   string  `json:"site_id"`
	ServerID string  `json:"server_id"`
	UserID   *string `json:"user_id,omitempty"`
}

// DisableLaravelReverbJob disables Laravel Reverb for a site
type DisableLaravelReverbJob struct {
	Deps    *JobDeps
	Payload DisableLaravelReverbPayload

	// Model fields for Failed() callback
	site   *models.Site
	server *servermodels.Server
}

// NewDisableLaravelReverbJob creates a new DisableLaravelReverbJob
func NewDisableLaravelReverbJob(p DisableLaravelReverbPayload) pkgjobs.Handler {
	return &DisableLaravelReverbJob{Deps: deps, Payload: p}
}

// Handle executes the disable Reverb job
func (j *DisableLaravelReverbJob) Handle(ctx context.Context) error {
	site, err := j.Deps.Repos.Site().FindByID(ctx, j.Payload.SiteID)
	if err != nil {
		return fmt.Errorf("failed to find site: %w", err)
	}
	j.site = site

	feature := site.GetEnabledFeature(FeatureReverb)
	if feature == nil || feature.DaemonID == nil {
		j.Deps.Logger.Info().Str("site_id", site.ID).Msg("Reverb not enabled, nothing to disable")
		return nil
	}

	daemonID := *feature.DaemonID

	server, err := j.Deps.ServerRepos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}
	j.server = server

	j.Deps.Logger.Info().
		Str("site_id", site.ID).
		Str("server_id", server.ID).
		Str("daemon_id", daemonID).
		Msg("Disabling Laravel Reverb")

	// Update site's enabled_features first (before dispatching uninstall)
	site.RemoveEnabledFeature(FeatureReverb)
	site.RemovePendingFeature(FeatureReverb)

	if err := j.Deps.Repos.Site().UpdateFields(ctx, site.ID, map[string]interface{}{
		"enabled_features": site.EnabledFeatures,
		"pending_features": site.PendingFeatures,
	}); err != nil {
		j.Deps.Logger.Error().Err(err).Msg("Failed to update site enabled_features")
	}

	// Dispatch UninstallDaemon job (handles supervisor config removal + record deletion)
	if err := j.dispatchUninstallDaemon(daemonID, server.ID); err != nil {
		j.Deps.Logger.Error().Err(err).Msg("Failed to dispatch uninstall daemon job")
	}

	// Update Caddyfile to remove WebSocket proxy block
	if err := j.dispatchUpdateCaddyfile(site.ID); err != nil {
		j.Deps.Logger.Error().Err(err).Msg("Failed to dispatch update Caddyfile job")
	}

	j.Deps.BroadcastServerEvent(server, "site.reverb_disabled", map[string]interface{}{
		"site_id": site.ID,
	})

	j.Deps.Logger.Info().Str("site_id", site.ID).Msg("Laravel Reverb disabled successfully")

	return nil
}

// dispatchUninstallDaemon dispatches the UninstallDaemon job
func (j *DisableLaravelReverbJob) dispatchUninstallDaemon(daemonID, serverID string) error {
	task, err := serverjobs.NewUninstallDaemonTask(serverID, daemonID, j.Payload.UserID)
	if err != nil {
		return err
	}
	return j.Deps.DispatchTask(task)
}

// dispatchUpdateCaddyfile dispatches the UpdateCaddyfile job
func (j *DisableLaravelReverbJob) dispatchUpdateCaddyfile(siteID string) error {
	task, err := NewUpdateCaddyfileTask(siteID, j.Payload.UserID)
	if err != nil {
		return err
	}
	return j.Deps.DispatchTask(task)
}

// Failed handles job failure
func (j *DisableLaravelReverbJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).Str("site_id", j.Payload.SiteID).Msg("Failed to disable Laravel Reverb")

	site, findErr := j.Deps.Repos.Site().FindByID(ctx, j.Payload.SiteID)
	if findErr == nil {
		site.RemovePendingFeature(FeatureReverb)
		if updateErr := j.Deps.Repos.Site().UpdateFields(ctx, site.ID, map[string]interface{}{
			"pending_features": site.PendingFeatures,
		}); updateErr != nil {
			j.Deps.Logger.Error().Err(updateErr).Str("site_id", site.ID).Msg("Failed to clear pending Reverb feature")
		}
	}
}

// NewDisableLaravelReverbTask creates a disable Reverb task
func NewDisableLaravelReverbTask(siteID, serverID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.Task(TypeDisableLaravelReverb, DisableLaravelReverbPayload{
		SiteID:   siteID,
		ServerID: serverID,
		UserID:   userID,
	})
}
