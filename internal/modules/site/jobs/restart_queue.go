package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/site/tasks"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeRestartQueue = "site:restart_queue"
const TypeRestartAllSiteQueues = "site:restart_all_queues"

// RestartQueuePayload holds data for queue worker restart
type RestartQueuePayload struct {
	SiteID  string  `json:"site_id"`
	QueueID string  `json:"queue_id"`
	UserID  *string `json:"user_id,omitempty"`
}

// RestartQueueJob handles queue worker restart
type RestartQueueJob struct {
	pkgjobs.BaseJob[*JobContext, RestartQueuePayload]
}

// NewRestartQueueJob creates a new RestartQueueJob with the given context and payload
func NewRestartQueueJob(ctx *JobContext, payload RestartQueuePayload) *RestartQueueJob {
	return &RestartQueueJob{
		BaseJob: pkgjobs.NewBaseJob(ctx, payload),
	}
}

// Handle executes the restart queue job
func (j *RestartQueueJob) Handle(ctx context.Context) error {
	// Get queue
	queue, err := j.Ctx.QueueRepo.FindByID(ctx, j.Payload.QueueID)
	if err != nil {
		return fmt.Errorf("failed to find queue: %w", err)
	}

	// Get site
	site, err := j.Ctx.SiteRepo.FindByID(ctx, j.Payload.SiteID)
	if err != nil {
		return fmt.Errorf("failed to find site: %w", err)
	}

	// Get server
	server, err := j.Ctx.ServerRepos.Server().FindByID(ctx, site.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	j.Ctx.LogInfo("Restarting queue worker",
		"queue_id", queue.ID,
		"site_id", site.ID,
	)

	// Restart the queue using supervisor
	restartTask := tasks.RestartQueue(queue.ID)

	result, err := j.Ctx.RunTaskOnServer(server, restartTask).AsRoot().Dispatch(ctx)
	if err != nil {
		j.Ctx.LogError(err, "Failed to restart queue", "queue_id", queue.ID)
		return err
	}

	if result.GetExitCode() != 0 {
		j.Ctx.LogError(nil, "Queue restart failed", "queue_id", queue.ID, "exit_code", result.GetExitCode())
		return fmt.Errorf("queue restart failed with exit code %d", result.GetExitCode())
	}

	// Broadcast success
	j.Ctx.BroadcastServerEvent(server, "queue.restarted", map[string]interface{}{
		"site_id":  site.ID,
		"queue_id": queue.ID,
	})

	j.Ctx.LogInfo("Queue worker restarted successfully", "queue_id", queue.ID)

	return nil
}

// Failed handles job failure
func (j *RestartQueueJob) Failed(ctx context.Context, err error) {
	j.Ctx.LogError(err, "Restart queue job failed",
		"site_id", j.Payload.SiteID,
		"queue_id", j.Payload.QueueID,
	)
}

// RestartAllSiteQueuesPayload holds data for restarting all queues for a site
type RestartAllSiteQueuesPayload struct {
	SiteID string  `json:"site_id"`
	UserID *string `json:"user_id,omitempty"`
}

// RestartAllSiteQueuesJob handles restarting all queues for a site
type RestartAllSiteQueuesJob struct {
	pkgjobs.BaseJob[*JobContext, RestartAllSiteQueuesPayload]
}

// NewRestartAllSiteQueuesJob creates a new RestartAllSiteQueuesJob with the given context and payload
func NewRestartAllSiteQueuesJob(ctx *JobContext, payload RestartAllSiteQueuesPayload) *RestartAllSiteQueuesJob {
	return &RestartAllSiteQueuesJob{
		BaseJob: pkgjobs.NewBaseJob(ctx, payload),
	}
}

// Handle executes the restart all queues job
func (j *RestartAllSiteQueuesJob) Handle(ctx context.Context) error {
	// Get site
	site, err := j.Ctx.SiteRepo.FindByID(ctx, j.Payload.SiteID)
	if err != nil {
		return fmt.Errorf("failed to find site: %w", err)
	}

	// Get all queues for the site
	queues, err := j.Ctx.QueueRepo.FindBySite(ctx, site.ID)
	if err != nil {
		return fmt.Errorf("failed to find queues: %w", err)
	}

	if len(queues) == 0 {
		j.Ctx.LogInfo("No queues to restart", "site_id", site.ID)
		return nil
	}

	// Get server
	server, err := j.Ctx.ServerRepos.Server().FindByID(ctx, site.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	j.Ctx.LogInfo("Restarting all queues for site",
		"site_id", site.ID,
		"queue_count", len(queues),
	)

	// Collect queue IDs
	queueIDs := make([]string, len(queues))
	for i, q := range queues {
		queueIDs[i] = q.ID
	}

	// Restart all queues
	restartTask := tasks.RestartAllQueues(queueIDs)

	result, err := j.Ctx.RunTaskOnServer(server, restartTask).AsRoot().Dispatch(ctx)
	if err != nil {
		j.Ctx.LogError(err, "Failed to restart queues", "site_id", site.ID)
		return err
	}

	if result.GetExitCode() != 0 {
		j.Ctx.LogError(nil, "Queue restart failed", "site_id", site.ID, "exit_code", result.GetExitCode())
	}

	// Broadcast success
	j.Ctx.BroadcastServerEvent(server, "queues.restarted", map[string]interface{}{
		"site_id":     site.ID,
		"queue_count": len(queues),
	})

	j.Ctx.LogInfo("All queues restarted successfully", "site_id", site.ID)

	return nil
}

// Failed handles job failure
func (j *RestartAllSiteQueuesJob) Failed(ctx context.Context, err error) {
	j.Ctx.LogError(err, "Restart all queues job failed", "site_id", j.Payload.SiteID)
}

// NewRestartQueueTask creates a restart queue job
func NewRestartQueueTask(siteID, queueID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.NewTask(TypeRestartQueue, RestartQueuePayload{
		SiteID:  siteID,
		QueueID: queueID,
		UserID:  userID,
	})
}

// NewRestartAllSiteQueuesTask creates a restart all site queues job
func NewRestartAllSiteQueuesTask(siteID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.NewTask(TypeRestartAllSiteQueues, RestartAllSiteQueuesPayload{
		SiteID: siteID,
		UserID: userID,
	})
}
