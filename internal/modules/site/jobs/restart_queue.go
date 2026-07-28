package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/site/models"
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
	Deps    *JobDeps
	Payload RestartQueuePayload

	// Model fields for Failed() callback
	site   *models.Site
	server *servermodels.Server
	queue  *models.Queue
}

// NewRestartQueueJob creates a new RestartQueueJob with the given payload
func NewRestartQueueJob(p RestartQueuePayload) pkgjobs.Handler {
	return &RestartQueueJob{Deps: deps, Payload: p}
}

// Handle executes the restart queue job
func (j *RestartQueueJob) Handle(ctx context.Context) error {
	// Get queue
	queue, err := j.Deps.Repos.Queue().FindByID(ctx, j.Payload.QueueID)
	if err != nil {
		return fmt.Errorf("failed to find queue: %w", err)
	}
	j.queue = queue

	// Get site
	site, err := j.Deps.Repos.Site().FindByID(ctx, j.Payload.SiteID)
	if err != nil {
		return fmt.Errorf("failed to find site: %w", err)
	}
	j.site = site

	// Get server
	server, err := j.Deps.ServerRepos.Server().FindByID(ctx, site.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}
	j.server = server

	j.Deps.Logger.Info().
		Str("queue_id", queue.ID).
		Str("site_id", site.ID).
		Msg("Restarting queue worker")

	// Restart the queue using supervisor
	restartTask := tasks.RestartQueue(queue.ID)

	result, err := j.Deps.RunTask(server, restartTask).AsRoot().Dispatch(ctx)
	if err != nil {
		j.Deps.Logger.Error().Err(err).Str("queue_id", queue.ID).Msg("Failed to restart queue")
		return err
	}
	if result.Error != nil {
		j.Deps.Logger.Error().Err(result.Error).Str("queue_id", queue.ID).Msg("Failed to restart queue")
		return result.Error
	}

	if result.GetExitCode() != 0 {
		j.Deps.Logger.Error().Str("queue_id", queue.ID).Int("exit_code", result.GetExitCode()).Msg("Queue restart failed")
		return fmt.Errorf("queue restart failed with exit code %d", result.GetExitCode())
	}

	// Broadcast success
	j.Deps.BroadcastServerEvent(server, "queue.restarted", map[string]interface{}{
		"site_id":  site.ID,
		"queue_id": queue.ID,
	})

	j.Deps.Logger.Info().Str("queue_id", queue.ID).Msg("Queue worker restarted successfully")

	return nil
}

// Failed handles job failure
func (j *RestartQueueJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).
		Str("site_id", j.Payload.SiteID).
		Str("queue_id", j.Payload.QueueID).
		Msg("Restart queue job failed")
}

// RestartAllSiteQueuesPayload holds data for restarting all queues for a site
type RestartAllSiteQueuesPayload struct {
	SiteID string  `json:"site_id"`
	UserID *string `json:"user_id,omitempty"`
}

// RestartAllSiteQueuesJob handles restarting all queues for a site
type RestartAllSiteQueuesJob struct {
	Deps    *JobDeps
	Payload RestartAllSiteQueuesPayload

	// Model fields for Failed() callback
	site   *models.Site
	server *servermodels.Server
}

// NewRestartAllSiteQueuesJob creates a new RestartAllSiteQueuesJob with the given payload
func NewRestartAllSiteQueuesJob(p RestartAllSiteQueuesPayload) pkgjobs.Handler {
	return &RestartAllSiteQueuesJob{Deps: deps, Payload: p}
}

// Handle executes the restart all queues job
func (j *RestartAllSiteQueuesJob) Handle(ctx context.Context) error {
	// Get site
	site, err := j.Deps.Repos.Site().FindByID(ctx, j.Payload.SiteID)
	if err != nil {
		return fmt.Errorf("failed to find site: %w", err)
	}
	j.site = site

	// Get all queues for the site
	queues, err := j.Deps.Repos.Queue().FindBySite(ctx, site.ID)
	if err != nil {
		return fmt.Errorf("failed to find queues: %w", err)
	}

	queueIDs := make([]string, 0, len(queues))
	for i := range queues {
		if queues[i].InstalledAt != nil && queues[i].InstallationFailedAt == nil {
			queueIDs = append(queueIDs, queues[i].ID)
		}
	}
	if len(queueIDs) == 0 {
		j.Deps.Logger.Info().Str("site_id", site.ID).Msg("No installed queues to restart")
		return nil
	}

	// Get server
	server, err := j.Deps.ServerRepos.Server().FindByID(ctx, site.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}
	j.server = server

	j.Deps.Logger.Info().
		Str("site_id", site.ID).
		Int("queue_count", len(queueIDs)).
		Msg("Restarting all queues for site")

	// Restart all queues
	restartTask := tasks.RestartAllQueues(queueIDs)

	result, err := j.Deps.RunTask(server, restartTask).AsRoot().Dispatch(ctx)
	if err != nil {
		j.Deps.Logger.Error().Err(err).Str("site_id", site.ID).Msg("Failed to restart queues")
		return err
	}
	if result.Error != nil {
		j.Deps.Logger.Error().Err(result.Error).Str("site_id", site.ID).Msg("Failed to restart queues")
		return result.Error
	}

	if result.GetExitCode() != 0 {
		j.Deps.Logger.Error().Str("site_id", site.ID).Int("exit_code", result.GetExitCode()).Msg("Queue restart failed")
		return fmt.Errorf("queue restart failed with exit code %d", result.GetExitCode())
	}

	// Broadcast success
	j.Deps.BroadcastServerEvent(server, "queues.restarted", map[string]interface{}{
		"site_id":     site.ID,
		"queue_count": len(queueIDs),
	})

	j.Deps.Logger.Info().Str("site_id", site.ID).Msg("All queues restarted successfully")

	return nil
}

// Failed handles job failure
func (j *RestartAllSiteQueuesJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).Str("site_id", j.Payload.SiteID).Msg("Restart all queues job failed")
}

// NewRestartQueueTask creates a restart queue job
func NewRestartQueueTask(siteID, queueID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.Task(TypeRestartQueue, RestartQueuePayload{
		SiteID:  siteID,
		QueueID: queueID,
		UserID:  userID,
	})
}

// NewRestartAllSiteQueuesTask creates a restart all site queues job
func NewRestartAllSiteQueuesTask(
	siteID string,
	userID *string,
	opts ...asynq.Option,
) (*asynq.Task, error) {
	return pkgjobs.Task(TypeRestartAllSiteQueues, RestartAllSiteQueuesPayload{
		SiteID: siteID,
		UserID: userID,
	}, opts...)
}
