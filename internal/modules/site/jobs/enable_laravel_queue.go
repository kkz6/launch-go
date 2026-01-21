package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
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
	FeatureJobHelpers
}

// NewEnableLaravelQueueJob creates a new EnableLaravelQueueJob
func NewEnableLaravelQueueJob(ctx *JobContext, payload EnableLaravelQueuePayload) *EnableLaravelQueueJob {
	return &EnableLaravelQueueJob{
		BaseJob:           pkgjobs.NewBaseJob(ctx, payload),
		FeatureJobHelpers: FeatureJobHelpers{Ctx: ctx},
	}
}

// Handle executes the enable queue job
func (j *EnableLaravelQueueJob) Handle(ctx context.Context) error {
	result, err := j.LoadAndValidate(ctx, j.Payload.SiteID, j.Payload.ServerID, FeatureQueue)
	if err != nil {
		return err
	}
	if result.AlreadyEnabled {
		return nil
	}

	site, server := result.Site, result.Server
	j.BaseJob.Ctx.LogInfo("Enabling Laravel queue worker", "site_id", site.ID, "server_id", server.ID)

	userID := j.GetUserID(j.Payload.UserID, site)
	queue, err := j.createQueueWithConfig(ctx, site, server, userID)
	if err != nil {
		return err
	}

	if err := j.DispatchInstallQueue(ctx, queue.ID, site.ID, j.Payload.UserID); err != nil {
		return err
	}

	j.EnableFeature(ctx, site, FeatureQueue, &queue.ID, nil)
	j.BroadcastFeatureEnabled(server, FeatureQueue, site.ID, queue.ID)
	j.BaseJob.Ctx.LogInfo("Laravel queue worker enabled successfully", "site_id", site.ID, "queue_id", queue.ID)

	return nil
}

// createQueueWithConfig creates a queue record with full queue worker configuration
func (j *EnableLaravelQueueJob) createQueueWithConfig(ctx context.Context, site *models.Site, server *servermodels.Server, userID string) (*models.Queue, error) {
	command := fmt.Sprintf("%s %s/artisan queue:work", site.GetPhpBinary(), site.GetApplicationDirectory())

	maxTries := 3
	maxMemory := 128
	maxSecondsPerJob := 60
	restSecondsOnEmpty := 3
	failedJobDelaySeconds := 3

	queue := &models.Queue{
		Command:               command,
		User:                  site.User,
		QueueConnection:       "database",
		QueueName:             "default",
		AutoStart:             true,
		AutoRestart:           true,
		NumProcs:              1,
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
	queue.SiteID = site.ID
	queue.ServerID = server.ID
	queue.UserID = userID

	if err := j.BaseJob.Ctx.QueueRepo.Create(ctx, queue); err != nil {
		return nil, fmt.Errorf("failed to create queue: %w", err)
	}

	return queue, nil
}

// Failed handles job failure
func (j *EnableLaravelQueueJob) Failed(ctx context.Context, err error) {
	j.HandleFailure(ctx, err, j.Payload.SiteID, FeatureQueue, "Failed to enable Laravel queue worker")
}

// NewEnableLaravelQueueTask creates an enable queue task
func NewEnableLaravelQueueTask(siteID, serverID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.NewTask(TypeEnableLaravelQueue, EnableLaravelQueuePayload{
		SiteID:   siteID,
		ServerID: serverID,
		UserID:   userID,
	})
}
