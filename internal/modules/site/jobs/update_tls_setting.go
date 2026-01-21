package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/site/enums"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeUpdateSiteTLSSetting = "site:update_tls_setting"

// UpdateSiteTLSSettingPayload holds data for updating TLS settings
type UpdateSiteTLSSettingPayload struct {
	SiteID     string           `json:"site_id"`
	TLSSetting enums.TLSSetting `json:"tls_setting"`
	UserID     *string          `json:"user_id,omitempty"`
}

// UpdateSiteTLSSettingJob updates TLS settings for a site
type UpdateSiteTLSSettingJob struct {
	pkgjobs.BaseJob[*JobContext, UpdateSiteTLSSettingPayload]
}

// NewUpdateSiteTLSSettingJob creates a new UpdateSiteTLSSettingJob
func NewUpdateSiteTLSSettingJob(ctx *JobContext, payload UpdateSiteTLSSettingPayload) *UpdateSiteTLSSettingJob {
	return &UpdateSiteTLSSettingJob{
		BaseJob: pkgjobs.NewBaseJob(ctx, payload),
	}
}

// Handle executes the update TLS setting job
func (j *UpdateSiteTLSSettingJob) Handle(ctx context.Context) error {
	site, err := j.Ctx.SiteRepo.FindByID(ctx, j.Payload.SiteID)
	if err != nil {
		return fmt.Errorf("failed to find site: %w", err)
	}

	oldTLSSetting := site.TLSSetting

	j.Ctx.LogInfo("Updating TLS setting",
		"site_id", site.ID,
		"old_setting", oldTLSSetting,
		"new_setting", j.Payload.TLSSetting,
	)

	// Update the TLS setting
	site.TLSSetting = j.Payload.TLSSetting
	now := time.Now()
	site.PendingTLSUpdateSince = &now

	if err := j.Ctx.SiteRepo.Update(ctx, site); err != nil {
		return fmt.Errorf("failed to update site TLS setting: %w", err)
	}

	// Dispatch UpdateCaddyfile job to apply the new TLS configuration
	task, err := NewUpdateCaddyfileTask(site.ID, j.Payload.UserID)
	if err != nil {
		return fmt.Errorf("failed to create update caddyfile task: %w", err)
	}

	if err := j.Ctx.DispatchTask(task); err != nil {
		return fmt.Errorf("failed to enqueue update caddyfile job: %w", err)
	}

	j.Ctx.LogInfo("TLS setting updated, Caddyfile update dispatched",
		"site_id", site.ID,
		"tls_setting", j.Payload.TLSSetting,
	)

	// Broadcast the update
	server, err := j.Ctx.ServerRepos.Server().FindByID(ctx, site.ServerID)
	if err == nil && server != nil {
		j.Ctx.BroadcastServerEvent(server, "site.tls_updated", map[string]interface{}{
			"site_id":     site.ID,
			"tls_setting": j.Payload.TLSSetting,
		})
	}

	return nil
}

// Failed handles job failure
func (j *UpdateSiteTLSSettingJob) Failed(ctx context.Context, err error) {
	j.Ctx.LogError(err, "Failed to update TLS setting",
		"site_id", j.Payload.SiteID,
		"tls_setting", j.Payload.TLSSetting,
	)

	// Clear pending TLS update flag on failure
	site, findErr := j.Ctx.SiteRepo.FindByID(ctx, j.Payload.SiteID)
	if findErr == nil && site != nil {
		site.PendingTLSUpdateSince = nil
		_ = j.Ctx.SiteRepo.Update(ctx, site)
	}
}

// NewUpdateSiteTLSSettingTask creates an update TLS setting task
func NewUpdateSiteTLSSettingTask(siteID string, tlsSetting enums.TLSSetting, userID *string) (*asynq.Task, error) {
	return pkgjobs.NewTask(TypeUpdateSiteTLSSetting, UpdateSiteTLSSettingPayload{
		SiteID:     siteID,
		TLSSetting: tlsSetting,
		UserID:     userID,
	})
}
