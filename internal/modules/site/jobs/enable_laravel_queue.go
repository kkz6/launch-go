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

const TypeEnableLaravelQueue = "site:enable_laravel_queue"

// EnableLaravelQueuePayload holds data for enabling Laravel queue
type EnableLaravelQueuePayload struct {
	SiteID   string  `json:"site_id"`
	ServerID string  `json:"server_id"`
	UserID   *string `json:"user_id,omitempty"`
}

// EnableLaravelQueueJob enables a Laravel queue worker for a site
type EnableLaravelQueueJob struct {
	pkgjobs.BaseJob[*JobContext, EnableLaravelQueuePayload]
}

// NewEnableLaravelQueueJob creates a new EnableLaravelQueueJob
func NewEnableLaravelQueueJob(ctx *JobContext, payload EnableLaravelQueuePayload) *EnableLaravelQueueJob {
	return &EnableLaravelQueueJob{
		BaseJob: pkgjobs.NewBaseJob(ctx, payload),
	}
}

// Handle executes the enable queue job
func (j *EnableLaravelQueueJob) Handle(ctx context.Context) error {
	// Get site
	site, err := j.Ctx.SiteRepo.FindByID(ctx, j.Payload.SiteID)
	if err != nil {
		return fmt.Errorf("failed to find site: %w", err)
	}

	// Check if site is Laravel type
	if site.Type != enums.SiteTypeLaravel {
		return fmt.Errorf("queue workers can only be enabled for Laravel sites")
	}

	// Check if queue is already enabled
	if site.HasEnabledFeature("queue") {
		j.Ctx.LogInfo("Queue already enabled", "site_id", site.ID)
		return nil
	}

	// Get server
	server, err := j.Ctx.ServerRepos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	j.Ctx.LogInfo("Enabling Laravel queue worker",
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

	// Build queue command
	command := j.buildQueueCommand(site)

	// Default queue configuration
	numProcs := 1
	maxTries := 3
	maxMemory := 128
	maxSecondsPerJob := 60
	restSecondsOnEmpty := 3
	failedJobDelaySeconds := 3

	// Create queue record
	queue := &models.Queue{
		SiteID:                site.ID,
		ServerID:              server.ID,
		UserID:                userID,
		Command:               command,
		User:                  site.User,
		QueueConnection:       "database",
		QueueName:             "default",
		AutoStart:             true,
		AutoRestart:           true,
		NumProcs:              numProcs,
		RedirectStderr:        true,
		StopWaitSeconds:       10,
		StopSignal:            "SIGTERM",
		MaxTries:              &maxTries,
		MaxMemory:             &maxMemory,
		MaxSecondsPerJob:      &maxSecondsPerJob,
		RestSecondsOnEmpty:    &restSecondsOnEmpty,
		FailedJobDelaySeconds: &failedJobDelaySeconds,
		RunOnMaintenance:      false,
		RunWithListen:         false,
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
		Name:      "queue",
		QueueID:   &queue.ID,
		EnabledAt: &now,
	}
	site.AddEnabledFeature(feature)
	site.RemovePendingFeature("queue")

	if err := j.Ctx.SiteRepo.UpdateFields(ctx, site.ID, map[string]interface{}{
		"enabled_features": site.EnabledFeatures,
		"pending_features": site.PendingFeatures,
	}); err != nil {
		j.Ctx.LogError(err, "Failed to update site enabled_features")
	}

	// Broadcast success
	j.Ctx.BroadcastServerEvent(server, "site.queue_enabled", map[string]interface{}{
		"site_id":  site.ID,
		"queue_id": queue.ID,
	})

	j.Ctx.LogInfo("Laravel queue worker enabled successfully",
		"site_id", site.ID,
		"queue_id", queue.ID,
	)

	return nil
}

// buildQueueCommand builds the artisan queue:work command
func (j *EnableLaravelQueueJob) buildQueueCommand(site *models.Site) string {
	return fmt.Sprintf("%s %s/artisan queue:work",
		site.GetPhpBinary(), site.GetApplicationDirectory())
}

// dispatchInstallQueue dispatches the InstallQueue job
func (j *EnableLaravelQueueJob) dispatchInstallQueue(queueID, siteID string) error {
	task, err := NewInstallQueueTask(siteID, queueID, j.Payload.UserID)
	if err != nil {
		return err
	}
	return j.Ctx.DispatchTask(task)
}

// Failed handles job failure
func (j *EnableLaravelQueueJob) Failed(ctx context.Context, err error) {
	j.Ctx.LogError(err, "Failed to enable Laravel queue worker",
		"site_id", j.Payload.SiteID,
	)

	// Remove from pending features
	site, findErr := j.Ctx.SiteRepo.FindByID(ctx, j.Payload.SiteID)
	if findErr == nil {
		site.RemovePendingFeature("queue")
		_ = j.Ctx.SiteRepo.UpdateFields(ctx, site.ID, map[string]interface{}{
			"pending_features": site.PendingFeatures,
		})
	}
}

// NewEnableLaravelQueueTask creates an enable queue task
func NewEnableLaravelQueueTask(siteID, serverID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.NewTask(TypeEnableLaravelQueue, EnableLaravelQueuePayload{
		SiteID:   siteID,
		ServerID: serverID,
		UserID:   userID,
	})
}
