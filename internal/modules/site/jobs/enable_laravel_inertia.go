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

const TypeEnableLaravelInertia = "site:enable_laravel_inertia"

// EnableLaravelInertiaPayload holds data for enabling Laravel Inertia SSR
type EnableLaravelInertiaPayload struct {
	SiteID   string  `json:"site_id"`
	ServerID string  `json:"server_id"`
	UserID   *string `json:"user_id,omitempty"`
}

// EnableLaravelInertiaJob enables Inertia SSR for a Laravel site
type EnableLaravelInertiaJob struct {
	pkgjobs.BaseJob[*JobContext, EnableLaravelInertiaPayload]
}

// NewEnableLaravelInertiaJob creates a new EnableLaravelInertiaJob
func NewEnableLaravelInertiaJob(ctx *JobContext, payload EnableLaravelInertiaPayload) *EnableLaravelInertiaJob {
	return &EnableLaravelInertiaJob{
		BaseJob: pkgjobs.NewBaseJob(ctx, payload),
	}
}

// Handle executes the enable Inertia job
func (j *EnableLaravelInertiaJob) Handle(ctx context.Context) error {
	// Get site
	site, err := j.Ctx.SiteRepo.FindByID(ctx, j.Payload.SiteID)
	if err != nil {
		return fmt.Errorf("failed to find site: %w", err)
	}

	// Check if site is Laravel type
	if site.Type != enums.SiteTypeLaravel {
		return fmt.Errorf("inertia SSR can only be enabled for Laravel sites")
	}

	// Check if Inertia is already enabled
	if site.HasEnabledFeature("inertia") {
		j.Ctx.LogInfo("Inertia SSR already enabled", "site_id", site.ID)
		return nil
	}

	// Get server
	server, err := j.Ctx.ServerRepos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	j.Ctx.LogInfo("Enabling Laravel Inertia SSR",
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

	// Build Inertia SSR command
	// Inertia SSR runs a Node.js server
	command := j.buildInertiaCommand(site)

	// Inertia SSR uses the daemon system (similar to queues but for Node processes)
	// Create queue record for Inertia SSR daemon
	queue := &models.Queue{
		SiteID:          site.ID,
		ServerID:        server.ID,
		UserID:          userID,
		Command:         command,
		User:            site.User,
		AutoStart:       true,
		AutoRestart:     true,
		NumProcs:        1,
		RedirectStderr:  true,
		StopWaitSeconds: 10,
		StopSignal:      "SIGTERM",
	}

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
		Name:      "inertia",
		QueueID:   &queue.ID,
		EnabledAt: &now,
	}
	site.AddEnabledFeature(feature)
	site.RemovePendingFeature("inertia")

	if err := j.Ctx.SiteRepo.UpdateFields(ctx, site.ID, map[string]interface{}{
		"enabled_features": site.EnabledFeatures,
		"pending_features": site.PendingFeatures,
	}); err != nil {
		j.Ctx.LogError(err, "Failed to update site enabled_features")
	}

	// Broadcast success
	j.Ctx.BroadcastServerEvent(server, "site.inertia_enabled", map[string]interface{}{
		"site_id":  site.ID,
		"queue_id": queue.ID,
	})

	j.Ctx.LogInfo("Laravel Inertia SSR enabled successfully",
		"site_id", site.ID,
		"queue_id", queue.ID,
	)

	return nil
}

// buildInertiaCommand builds the Inertia SSR command
func (j *EnableLaravelInertiaJob) buildInertiaCommand(site *models.Site) string {
	// Inertia SSR runs via Node.js
	return fmt.Sprintf("node %s/bootstrap/ssr/ssr.js",
		site.GetApplicationDirectory())
}

// dispatchInstallQueue dispatches the InstallQueue job
func (j *EnableLaravelInertiaJob) dispatchInstallQueue(queueID, siteID string) error {
	if j.Ctx.Queue == nil {
		return fmt.Errorf("queue client not available")
	}

	task, err := NewInstallQueueTask(siteID, queueID, j.Payload.UserID)
	if err != nil {
		return err
	}

	_, err = j.Ctx.Queue.Enqueue(task)
	return err
}

// Failed handles job failure
func (j *EnableLaravelInertiaJob) Failed(ctx context.Context, err error) {
	j.Ctx.LogError(err, "Failed to enable Laravel Inertia SSR",
		"site_id", j.Payload.SiteID,
	)

	// Remove from pending features
	site, findErr := j.Ctx.SiteRepo.FindByID(ctx, j.Payload.SiteID)
	if findErr == nil {
		site.RemovePendingFeature("inertia")
		_ = j.Ctx.SiteRepo.UpdateFields(ctx, site.ID, map[string]interface{}{
			"pending_features": site.PendingFeatures,
		})
	}
}

// NewEnableLaravelInertiaTask creates an enable Inertia task
func NewEnableLaravelInertiaTask(siteID, serverID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.NewTask(TypeEnableLaravelInertia, EnableLaravelInertiaPayload{
		SiteID:   siteID,
		ServerID: serverID,
		UserID:   userID,
	})
}
