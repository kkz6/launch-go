package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/site/enums"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeUpdateSiteTlsSetting = "site:update_tls_setting"

// UpdateSiteTlsSettingPayload holds data for updating TLS settings
type UpdateSiteTlsSettingPayload struct {
	SiteID     string          `json:"site_id"`
	TlsSetting enums.TlsSetting `json:"tls_setting"`
	UserID     *string         `json:"user_id,omitempty"`
}

// UpdateSiteTlsSettingJob updates TLS settings for a site
type UpdateSiteTlsSettingJob struct {
	ctx     *JobContext
	Payload UpdateSiteTlsSettingPayload
}

// NewUpdateSiteTlsSettingJob creates a new UpdateSiteTlsSettingJob
func NewUpdateSiteTlsSettingJob(ctx *JobContext, payload UpdateSiteTlsSettingPayload) *UpdateSiteTlsSettingJob {
	return &UpdateSiteTlsSettingJob{
		ctx:     ctx,
		Payload: payload,
	}
}

// Handle executes the update TLS setting job
func (j *UpdateSiteTlsSettingJob) Handle(ctx context.Context) error {
	site, err := j.ctx.SiteRepo.FindByID(ctx, j.Payload.SiteID)
	if err != nil {
		return fmt.Errorf("failed to find site: %w", err)
	}

	oldTlsSetting := site.TlsSetting

	j.ctx.LogInfo("Updating TLS setting",
		"site_id", site.ID,
		"old_setting", oldTlsSetting,
		"new_setting", j.Payload.TlsSetting,
	)

	// Update the TLS setting
	site.TlsSetting = j.Payload.TlsSetting
	now := time.Now()
	site.PendingTlsUpdateSince = &now

	if err := j.ctx.SiteRepo.Update(ctx, site); err != nil {
		return fmt.Errorf("failed to update site TLS setting: %w", err)
	}

	// Dispatch UpdateCaddyfile job to apply the new TLS configuration
	task, err := NewUpdateCaddyfileTask(site.ID, j.Payload.UserID)
	if err != nil {
		return fmt.Errorf("failed to create update caddyfile task: %w", err)
	}

	if j.ctx.Queue != nil {
		if _, err := j.ctx.Queue.Enqueue(task); err != nil {
			return fmt.Errorf("failed to enqueue update caddyfile job: %w", err)
		}
	}

	j.ctx.LogInfo("TLS setting updated, Caddyfile update dispatched",
		"site_id", site.ID,
		"tls_setting", j.Payload.TlsSetting,
	)

	// Broadcast the update
	server, err := j.ctx.ServerRepos.Server().FindByID(ctx, site.ServerID)
	if err == nil && server != nil {
		j.ctx.BroadcastServerEvent(server, "site.tls_updated", map[string]interface{}{
			"site_id":     site.ID,
			"tls_setting": j.Payload.TlsSetting,
		})
	}

	return nil
}

// Failed handles job failure
func (j *UpdateSiteTlsSettingJob) Failed(ctx context.Context, err error) {
	j.ctx.LogError(err, "Failed to update TLS setting",
		"site_id", j.Payload.SiteID,
		"tls_setting", j.Payload.TlsSetting,
	)

	// Clear pending TLS update flag on failure
	site, findErr := j.ctx.SiteRepo.FindByID(ctx, j.Payload.SiteID)
	if findErr == nil && site != nil {
		site.PendingTlsUpdateSince = nil
		_ = j.ctx.SiteRepo.Update(ctx, site)
	}
}

// NewUpdateSiteTlsSettingTask creates an update TLS setting task
func NewUpdateSiteTlsSettingTask(siteID string, tlsSetting enums.TlsSetting, userID *string) (*asynq.Task, error) {
	return pkgjobs.NewTask(TypeUpdateSiteTlsSetting, UpdateSiteTlsSettingPayload{
		SiteID:     siteID,
		TlsSetting: tlsSetting,
		UserID:     userID,
	})
}
