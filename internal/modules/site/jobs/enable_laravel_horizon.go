package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/hibiken/asynq"

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
	pkgjobs.BaseJob[*JobContext, EnableLaravelHorizonPayload]
}

// NewEnableLaravelHorizonJob creates a new EnableLaravelHorizonJob
func NewEnableLaravelHorizonJob(ctx *JobContext, payload EnableLaravelHorizonPayload) *EnableLaravelHorizonJob {
	return &EnableLaravelHorizonJob{
		BaseJob: pkgjobs.NewBaseJob(ctx, payload),
	}
}

// Handle executes the enable Horizon job
func (j *EnableLaravelHorizonJob) Handle(ctx context.Context) error {
	// Get site
	site, err := j.Ctx.SiteRepo.FindByID(ctx, j.Payload.SiteID)
	if err != nil {
		return fmt.Errorf("failed to find site: %w", err)
	}

	// Check if site is Laravel type
	if site.Type != enums.SiteTypeLaravel {
		return fmt.Errorf("horizon can only be enabled for Laravel sites")
	}

	// Check if Horizon is already enabled
	if site.HasEnabledFeature("horizon") {
		j.Ctx.LogInfo("Horizon already enabled", "site_id", site.ID)
		return nil
	}

	// Check if queue is enabled (they conflict)
	if site.HasEnabledFeature("queue") {
		return fmt.Errorf("cannot enable Horizon while queue workers are enabled - disable queue workers first")
	}

	// Get server
	server, err := j.Ctx.ServerRepos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	j.Ctx.LogInfo("Enabling Laravel Horizon",
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
		Command:         command,
		User:            site.User,
		AutoStart:       true,
		AutoRestart:     true,
		NumProcs:        numProcs,
		RedirectStderr:  true,
		StopWaitSeconds: 10,
		StopSignal:      "SIGTERM",
	}
	queue.SiteID = site.ID
	queue.ServerID = server.ID
	queue.UserID = userID

	if err := j.Ctx.QueueRepo.Create(ctx, queue); err != nil {
		return fmt.Errorf("failed to create queue: %w", err)
	}

	// Dispatch InstallQueue job
	if err := j.dispatchInstallQueue(queue.ID, site.ID); err != nil {
		// Cleanup the queue record if dispatch fails
		_ = j.Ctx.QueueRepo.Delete(ctx, queue.ID)
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

	if err := j.Ctx.SiteRepo.UpdateFields(ctx, site.ID, map[string]interface{}{
		"enabled_features": site.EnabledFeatures,
		"pending_features": site.PendingFeatures,
	}); err != nil {
		j.Ctx.LogError(err, "Failed to update site enabled_features")
	}

	// Broadcast success
	j.Ctx.BroadcastServerEvent(server, "site.horizon_enabled", map[string]interface{}{
		"site_id":  site.ID,
		"queue_id": queue.ID,
	})

	j.Ctx.LogInfo("Laravel Horizon enabled successfully",
		"site_id", site.ID,
		"queue_id", queue.ID,
	)

	return nil
}

// buildHorizonCommand builds the artisan horizon command
func (j *EnableLaravelHorizonJob) buildHorizonCommand(site *models.Site) string {
	return fmt.Sprintf("%s %s/artisan horizon",
		site.GetPhpBinary(), site.GetApplicationDirectory())
}

// dispatchInstallQueue dispatches the InstallQueue job
func (j *EnableLaravelHorizonJob) dispatchInstallQueue(queueID, siteID string) error {
	task, err := NewInstallQueueTask(siteID, queueID, j.Payload.UserID)
	if err != nil {
		return err
	}
	return j.Ctx.DispatchTask(task)
}

// Failed handles job failure
func (j *EnableLaravelHorizonJob) Failed(ctx context.Context, err error) {
	j.Ctx.LogError(err, "Failed to enable Laravel Horizon",
		"site_id", j.Payload.SiteID,
	)

	// Remove from pending features
	site, findErr := j.Ctx.SiteRepo.FindByID(ctx, j.Payload.SiteID)
	if findErr == nil {
		site.RemovePendingFeature("horizon")
		_ = j.Ctx.SiteRepo.UpdateFields(ctx, site.ID, map[string]interface{}{
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
