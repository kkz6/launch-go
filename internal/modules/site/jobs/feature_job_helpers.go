package jobs

import (
	"context"
	"fmt"
	"time"

	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	sitetypes "github.com/kkz6/launch-go/internal/modules/site/types"
)

// Laravel feature names
const (
	FeatureQueue     = "queue"
	FeatureScheduler = "scheduler"
	FeatureHorizon   = "horizon"
	FeatureInertia   = "inertia"
)

// FeatureJobHelpers provides common functionality for Laravel feature enable jobs
type FeatureJobHelpers struct {
	Ctx *JobContext
}

// FeatureLoadResult contains the result of loading and validating resources for a feature job
type FeatureLoadResult struct {
	Site           *models.Site
	Server         *servermodels.Server
	AlreadyEnabled bool
}

// LoadAndValidate validates the site is Laravel type and loads site and server.
// Returns AlreadyEnabled=true if the feature is already enabled (caller should return nil).
func (h *FeatureJobHelpers) LoadAndValidate(ctx context.Context, siteID, serverID, featureName string) (*FeatureLoadResult, error) {
	site, err := h.Ctx.SiteRepo.FindByID(ctx, siteID)
	if err != nil {
		return nil, fmt.Errorf("failed to find site: %w", err)
	}

	if site.Type != sitetypes.SiteTypeLaravel {
		return nil, fmt.Errorf("%s can only be enabled for Laravel sites", featureName)
	}

	if site.HasEnabledFeature(featureName) {
		h.Ctx.LogInfo(fmt.Sprintf("%s already enabled", featureName), "site_id", site.ID)
		return &FeatureLoadResult{Site: site, AlreadyEnabled: true}, nil
	}

	server, err := h.Ctx.ServerRepos.Server().FindByID(ctx, serverID)
	if err != nil {
		return nil, fmt.Errorf("failed to find server: %w", err)
	}

	return &FeatureLoadResult{Site: site, Server: server}, nil
}

// GetUserID returns the user ID from payload or falls back to site's user ID
func (h *FeatureJobHelpers) GetUserID(payloadUserID *string, site *models.Site) string {
	if payloadUserID != nil {
		return *payloadUserID
	}
	return site.UserID
}

// CreateQueueRecord creates a queue record with common defaults
func (h *FeatureJobHelpers) CreateQueueRecord(ctx context.Context, site *models.Site, server *servermodels.Server, command, userID string) (*models.Queue, error) {
	queue := &models.Queue{
		Command:         command,
		User:            site.User,
		AutoStart:       true,
		AutoRestart:     true,
		NumProcs:        1,
		RedirectStderr:  true,
		StopWaitSeconds: 10,
		StopSignal:      "SIGTERM",
	}
	queue.SiteID = site.ID
	queue.ServerID = server.ID
	queue.UserID = userID

	if err := h.Ctx.QueueRepo.Create(ctx, queue); err != nil {
		return nil, fmt.Errorf("failed to create queue: %w", err)
	}

	return queue, nil
}

// DispatchInstallQueue dispatches the InstallQueue job and cleans up on failure
func (h *FeatureJobHelpers) DispatchInstallQueue(ctx context.Context, queueID, siteID string, userID *string) error {
	task, err := NewInstallQueueTask(siteID, queueID, userID)
	if err != nil {
		return err
	}

	if err := h.Ctx.DispatchTask(task); err != nil {
		// Cleanup the queue record if dispatch fails
		_ = h.Ctx.QueueRepo.Delete(ctx, queueID)
		return fmt.Errorf("failed to dispatch install queue job: %w", err)
	}

	return nil
}

// EnableFeature updates the site's enabled features and removes from pending
func (h *FeatureJobHelpers) EnableFeature(ctx context.Context, site *models.Site, featureName string, queueID, cronID *string) {
	now := time.Now()
	feature := models.EnabledFeature{
		Name:      featureName,
		QueueID:   queueID,
		CronID:    cronID,
		EnabledAt: &now,
	}

	site.AddEnabledFeature(feature)
	site.RemovePendingFeature(featureName)

	if err := h.Ctx.SiteRepo.UpdateFields(ctx, site.ID, map[string]interface{}{
		"enabled_features": site.EnabledFeatures,
		"pending_features": site.PendingFeatures,
	}); err != nil {
		h.Ctx.LogError(err, "Failed to update site enabled_features")
	}
}

// BroadcastFeatureEnabled broadcasts a feature enabled event with queue_id
func (h *FeatureJobHelpers) BroadcastFeatureEnabled(server *servermodels.Server, featureName, siteID, queueID string) {
	h.Ctx.BroadcastServerEvent(server, fmt.Sprintf("site.%s_enabled", featureName), map[string]interface{}{
		"site_id":  siteID,
		"queue_id": queueID,
	})
}

// BroadcastSchedulerEnabled broadcasts the scheduler enabled event with cron_id
func (h *FeatureJobHelpers) BroadcastSchedulerEnabled(server *servermodels.Server, siteID, cronID string) {
	h.Ctx.BroadcastServerEvent(server, "site.scheduler_enabled", map[string]interface{}{
		"site_id": siteID,
		"cron_id": cronID,
	})
}

// HandleFailure removes the feature from pending features when a job fails
func (h *FeatureJobHelpers) HandleFailure(ctx context.Context, err error, siteID, featureName, logMessage string) {
	h.Ctx.LogError(err, logMessage, "site_id", siteID)

	site, findErr := h.Ctx.SiteRepo.FindByID(ctx, siteID)
	if findErr != nil {
		return
	}

	site.RemovePendingFeature(featureName)
	_ = h.Ctx.SiteRepo.UpdateFields(ctx, site.ID, map[string]interface{}{
		"pending_features": site.PendingFeatures,
	})
}
