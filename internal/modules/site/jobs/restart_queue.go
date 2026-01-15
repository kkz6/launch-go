package jobs

import (
	"context"
	"fmt"

	"github.com/kkz6/launch-go/internal/modules/site/tasks"
)

// RestartQueueJob handles queue worker restart
type RestartQueueJob struct {
	SiteJobBase
	Payload RestartQueuePayload
}

// Type returns the job type
func (j *RestartQueueJob) Type() string {
	return TypeRestartQueue
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
	server, err := j.Ctx.ServerRepo.FindServerByID(ctx, site.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	j.LogInfo("Restarting queue worker",
		"queue_id", queue.ID,
		"site_id", site.ID,
	)

	// Restart the queue using supervisor
	restartTask := tasks.RestartQueue(queue.ID)

	result, err := j.RunTaskOnServer(server, restartTask).AsRoot().Dispatch(ctx)
	if err != nil {
		j.LogError(err, "Failed to restart queue", "queue_id", queue.ID)
		return err
	}

	if result.GetExitCode() != 0 {
		j.LogError(nil, "Queue restart failed", "queue_id", queue.ID, "exit_code", result.GetExitCode())
		return fmt.Errorf("queue restart failed with exit code %d", result.GetExitCode())
	}

	// Broadcast success
	j.BroadcastSiteEvent(site.ID, "queue.restarted", map[string]interface{}{
		"site_id":  site.ID,
		"queue_id": queue.ID,
	})

	j.LogInfo("Queue worker restarted successfully", "queue_id", queue.ID)

	return nil
}

// Failed handles job failure
func (j *RestartQueueJob) Failed(ctx context.Context, err error) {
	j.LogError(err, "Restart queue job failed",
		"site_id", j.Payload.SiteID,
		"queue_id", j.Payload.QueueID,
	)
}

// RestartAllSiteQueuesJob handles restarting all queues for a site
type RestartAllSiteQueuesJob struct {
	SiteJobBase
	Payload struct {
		SiteID string  `json:"site_id"`
		UserID *string `json:"user_id,omitempty"`
	}
}

// Type returns the job type
func (j *RestartAllSiteQueuesJob) Type() string {
	return "site:restart_all_queues"
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
		j.LogInfo("No queues to restart", "site_id", site.ID)
		return nil
	}

	// Get server
	server, err := j.Ctx.ServerRepo.FindServerByID(ctx, site.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	j.LogInfo("Restarting all queues for site",
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

	result, err := j.RunTaskOnServer(server, restartTask).AsRoot().Dispatch(ctx)
	if err != nil {
		j.LogError(err, "Failed to restart queues", "site_id", site.ID)
		return err
	}

	if result.GetExitCode() != 0 {
		j.LogError(nil, "Queue restart failed", "site_id", site.ID, "exit_code", result.GetExitCode())
	}

	// Broadcast success
	j.BroadcastSiteEvent(site.ID, "queues.restarted", map[string]interface{}{
		"site_id":     site.ID,
		"queue_count": len(queues),
	})

	j.LogInfo("All queues restarted successfully", "site_id", site.ID)

	return nil
}

// Failed handles job failure
func (j *RestartAllSiteQueuesJob) Failed(ctx context.Context, err error) {
	j.LogError(err, "Restart all queues job failed", "site_id", j.Payload.SiteID)
}
