package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/hibiken/asynq"

	serverenums "github.com/kkz6/launch-go/internal/modules/server/enums"
	"github.com/kkz6/launch-go/internal/modules/site/enums"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeEnableLaravelHorizon = "site:enable_laravel_horizon"

// EnableLaravelHorizonPayload holds data for enabling Laravel Horizon
type EnableLaravelHorizonPayload struct {
	SiteID   string  `json:"site_id"`
	ServerID string  `json:"server_id"`
	UserID   *string `json:"user_id,omitempty"`
}

// EnableLaravelHorizonJob enables Laravel Horizon for a site
type EnableLaravelHorizonJob struct {
	ctx     *JobContext
	Payload EnableLaravelHorizonPayload
}

// NewEnableLaravelHorizonJob creates a new EnableLaravelHorizonJob
func NewEnableLaravelHorizonJob(ctx *JobContext, payload EnableLaravelHorizonPayload) *EnableLaravelHorizonJob {
	return &EnableLaravelHorizonJob{
		ctx:     ctx,
		Payload: payload,
	}
}

// Handle executes the enable Horizon job
func (j *EnableLaravelHorizonJob) Handle(ctx context.Context) error {
	// Get site
	site, err := j.ctx.SiteRepo.FindByID(ctx, j.Payload.SiteID)
	if err != nil {
		return fmt.Errorf("failed to find site: %w", err)
	}

	// Check if site is Laravel type
	if site.Type != enums.SiteTypeLaravel {
		return fmt.Errorf("Horizon can only be enabled for Laravel sites")
	}

	// Check if Horizon is already enabled
	if site.HasEnabledFeature("horizon") {
		j.ctx.LogInfo("Horizon already enabled", "site_id", site.ID)
		return nil
	}

	// Check if queue is enabled (they conflict)
	if site.HasEnabledFeature("queue") {
		return fmt.Errorf("cannot enable Horizon while queue workers are enabled - disable queue workers first")
	}

	// Get server
	server, err := j.ctx.ServerRepos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	j.ctx.LogInfo("Enabling Laravel Horizon",
		"site_id", site.ID,
		"server_id", server.ID,
	)

	// Get user ID
	userID := ""
	if j.Payload.UserID != nil {
		userID = *j.Payload.UserID
	} else {
		userID = site.UserID
	}

	// Build Horizon command
	command := j.buildHorizonCommand(site)

	// Horizon uses supervisor/queue system but with the horizon command
	numProcs := 1

	// Create queue record for Horizon daemon
	queue := &models.Queue{
		SiteID:         site.ID,
		ServerID:       server.ID,
		UserID:         userID,
		Command:        command,
		User:           site.User,
		AutoStart:      true,
		AutoRestart:    true,
		NumProcs:       numProcs,
		RedirectStderr: true,
		StopWaitSeconds: 10,
		StopSignal:     "SIGTERM",
	}

	if err := j.ctx.QueueRepo.Create(ctx, queue); err != nil {
		return fmt.Errorf("failed to create queue: %w", err)
	}

	// Dispatch InstallQueue job
	if err := j.dispatchInstallQueue(queue.ID, site.ID); err != nil {
		// Cleanup the queue record if dispatch fails
		_ = j.ctx.QueueRepo.Delete(ctx, queue.ID)
		return fmt.Errorf("failed to dispatch install queue job: %w", err)
	}

	// Update site's enabled_features
	now := time.Now()
	feature := models.EnabledFeature{
		Name:      "horizon",
		QueueID:   &queue.ID,
		EnabledAt: &now,
	}
	site.AddEnabledFeature(feature)
	site.RemovePendingFeature("horizon")

	if err := j.ctx.SiteRepo.UpdateFields(ctx, site.ID, map[string]interface{}{
		"enabled_features": site.EnabledFeatures,
		"pending_features": site.PendingFeatures,
	}); err != nil {
		j.ctx.LogError(err, "Failed to update site enabled_features")
	}

	// Broadcast success
	j.ctx.BroadcastServerEvent(server, "site.horizon_enabled", map[string]interface{}{
		"site_id":  site.ID,
		"queue_id": queue.ID,
	})

	j.ctx.LogInfo("Laravel Horizon enabled successfully",
		"site_id", site.ID,
		"queue_id", queue.ID,
	)

	return nil
}

// buildHorizonCommand builds the artisan horizon command
func (j *EnableLaravelHorizonJob) buildHorizonCommand(site *models.Site) string {
	phpVersion := ""
	if site.PhpVersion != nil {
		phpVersion = *site.PhpVersion
	}
	phpBinary := serverenums.PhpBinaryFromVersion(phpVersion)

	return fmt.Sprintf("%s %s/artisan horizon",
		phpBinary, site.GetApplicationDirectory())
}

// dispatchInstallQueue dispatches the InstallQueue job
func (j *EnableLaravelHorizonJob) dispatchInstallQueue(queueID, siteID string) error {
	if j.ctx.Queue == nil {
		return fmt.Errorf("queue client not available")
	}

	task, err := NewInstallQueueTask(siteID, queueID, j.Payload.UserID)
	if err != nil {
		return err
	}

	_, err = j.ctx.Queue.Enqueue(task)
	return err
}

// Failed handles job failure
func (j *EnableLaravelHorizonJob) Failed(ctx context.Context, err error) {
	j.ctx.LogError(err, "Failed to enable Laravel Horizon",
		"site_id", j.Payload.SiteID,
	)

	// Remove from pending features
	site, findErr := j.ctx.SiteRepo.FindByID(ctx, j.Payload.SiteID)
	if findErr == nil {
		site.RemovePendingFeature("horizon")
		_ = j.ctx.SiteRepo.UpdateFields(ctx, site.ID, map[string]interface{}{
			"pending_features": site.PendingFeatures,
		})
	}
}

// NewEnableLaravelHorizonTask creates an enable Horizon task
func NewEnableLaravelHorizonTask(siteID, serverID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.NewTask(TypeEnableLaravelHorizon, EnableLaravelHorizonPayload{
		SiteID:   siteID,
		ServerID: serverID,
		UserID:   userID,
	})
}
