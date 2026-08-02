package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/hibiken/asynq"

	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	sitetypes "github.com/kkz6/launch-go/internal/modules/site/types"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeUpdateSiteTLSSetting = "site:update_tls_setting"

// UpdateSiteTLSSettingPayload holds data for updating TLS settings
type UpdateSiteTLSSettingPayload struct {
	SiteID     string               `json:"site_id"`
	TLSSetting sitetypes.TLSSetting `json:"tls_setting"`
	UserID     *string              `json:"user_id,omitempty"`
}

// UpdateSiteTLSSettingJob updates TLS settings for a site
type UpdateSiteTLSSettingJob struct {
	Deps    *JobDeps
	Payload UpdateSiteTLSSettingPayload

	// Model fields for Failed() callback
	site   *models.Site
	server *servermodels.Server
}

// NewUpdateSiteTLSSettingJob creates a new UpdateSiteTLSSettingJob
func NewUpdateSiteTLSSettingJob(p UpdateSiteTLSSettingPayload) pkgjobs.Handler {
	return &UpdateSiteTLSSettingJob{Deps: deps, Payload: p}
}

// Handle executes the update TLS setting job
func (j *UpdateSiteTLSSettingJob) Handle(ctx context.Context) error {
	site, err := j.Deps.Repos.Site().FindByID(ctx, j.Payload.SiteID)
	if err != nil {
		return fmt.Errorf("failed to find site: %w", err)
	}
	j.site = site

	oldTLSSetting := site.TLSSetting
	certificates, err := j.Deps.Repos.Certificate().FindBySite(ctx, site.ID)
	if err != nil {
		return fmt.Errorf("failed to load active certificates: %w", err)
	}
	previousCertificateIDs := make([]string, 0, len(certificates))
	for i := range certificates {
		if certificates[i].IsActive {
			previousCertificateIDs = append(previousCertificateIDs, certificates[i].ID)
		}
	}

	j.Deps.Logger.Info().
		Str("site_id", site.ID).
		Str("old_setting", string(oldTLSSetting)).
		Str("new_setting", string(j.Payload.TLSSetting)).
		Msg("Updating TLS setting")

	// Update the TLS setting
	site.TLSSetting = j.Payload.TLSSetting
	now := time.Now()
	site.PendingTLSUpdateSince = &now
	site.PendingTLSPreviousSetting = &oldTLSSetting
	site.PendingTLSPreviousCertIDs = previousCertificateIDs
	site.PendingTLSReplacementCertID = nil

	if err := j.Deps.Repos.Site().Update(ctx, site); err != nil {
		return fmt.Errorf("failed to update site TLS setting: %w", err)
	}

	// Dispatch UpdateCaddyfile job to apply the new TLS configuration
	task, err := NewTLSUpdateCaddyfileTask(site.ID, j.Payload.UserID)
	if err != nil {
		return fmt.Errorf("failed to create update caddyfile task: %w", err)
	}

	if err := j.Deps.DispatchTask(task); err != nil {
		return fmt.Errorf("failed to enqueue update caddyfile job: %w", err)
	}

	j.Deps.Logger.Info().
		Str("site_id", site.ID).
		Str("tls_setting", string(j.Payload.TLSSetting)).
		Msg("TLS setting updated, Caddyfile update dispatched")

	// Broadcast the update
	server, err := j.Deps.ServerRepos.Server().FindByID(ctx, site.ServerID)
	if err == nil && server != nil {
		j.server = server
		j.Deps.BroadcastServerEvent(server, "site.tls_updated", map[string]interface{}{
			"site_id":     site.ID,
			"tls_setting": j.Payload.TLSSetting,
		})
	}

	return nil
}

// Failed handles job failure
func (j *UpdateSiteTLSSettingJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).
		Str("site_id", j.Payload.SiteID).
		Str("tls_setting", string(j.Payload.TLSSetting)).
		Msg("Failed to update TLS setting")

	cleanupCtx, cancelCleanup := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
	defer cancelCleanup()
	if rollbackErr := rollbackTLSReservation(cleanupCtx, j.Deps.DB, j.Payload.SiteID); rollbackErr != nil {
		j.Deps.Logger.Error().
			Err(rollbackErr).
			Str("site_id", j.Payload.SiteID).
			Msg("Failed to roll back TLS setting")
	}
}

// NewUpdateSiteTLSSettingTask creates an update TLS setting task
func NewUpdateSiteTLSSettingTask(siteID string, tlsSetting sitetypes.TLSSetting, userID *string) (*asynq.Task, error) {
	return pkgjobs.Task(TypeUpdateSiteTLSSetting, UpdateSiteTLSSettingPayload{
		SiteID:     siteID,
		TLSSetting: tlsSetting,
		UserID:     userID,
	})
}
