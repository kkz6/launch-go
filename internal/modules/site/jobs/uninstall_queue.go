package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/hibiken/asynq"

	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	"github.com/kkz6/launch-go/internal/modules/site/tasks"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeUninstallQueue = "site:uninstall_queue"

// UninstallQueuePayload holds data for queue worker uninstallation
type UninstallQueuePayload struct {
	SiteID  string  `json:"site_id"`
	QueueID string  `json:"queue_id"`
	UserID  *string `json:"user_id,omitempty"`
}

// UninstallQueueJob handles queue worker uninstallation
type UninstallQueueJob struct {
	Deps    *JobDeps
	Payload UninstallQueuePayload

	// Model fields for Failed() callback
	site   *models.Site
	server *servermodels.Server
	queue  *models.Queue
}

// NewUninstallQueueJob creates a new UninstallQueueJob with the given payload
func NewUninstallQueueJob(p UninstallQueuePayload) pkgjobs.Handler {
	return &UninstallQueueJob{Deps: deps, Payload: p}
}

// Handle executes the uninstall queue job
func (j *UninstallQueueJob) Handle(ctx context.Context) error {
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
		Msg("Uninstalling queue worker")

	// Delete the queue config and stop the process
	deleteTask := tasks.DeleteQueueConfig(queue.GetPath(), queue.ID)

	result, err := j.Deps.RunTask(server, deleteTask).AsUser().Dispatch(ctx)
	if err != nil {
		j.Deps.Logger.Error().Err(err).Str("queue_id", queue.ID).Msg("Failed to delete queue config")
		return err
	}

	if result.GetExitCode() != 0 {
		j.Deps.Logger.Error().Str("queue_id", queue.ID).Int("exit_code", result.GetExitCode()).Msg("Queue deletion failed")
	}

	// Delete the queue record from database
	if err := j.Deps.Repos.Queue().Delete(ctx, queue.ID); err != nil {
		j.Deps.Logger.Error().Err(err).Str("queue_id", queue.ID).Msg("Failed to delete queue record")
		return err
	}

	// Broadcast success
	j.Deps.BroadcastServerEvent(server, "queue.uninstalled", map[string]interface{}{
		"site_id":  site.ID,
		"queue_id": queue.ID,
	})

	j.Deps.Logger.Info().Str("queue_id", queue.ID).Msg("Queue worker uninstalled successfully")

	return nil
}

// Failed handles job failure
func (j *UninstallQueueJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).
		Str("site_id", j.Payload.SiteID).
		Str("queue_id", j.Payload.QueueID).
		Msg("Uninstall queue job failed")

	// Mark uninstallation as failed
	queue, findErr := j.Deps.Repos.Queue().FindByID(ctx, j.Payload.QueueID)
	if findErr != nil {
		return
	}

	now := time.Now()
	queue.UninstallationFailedAt = &now
	_ = j.Deps.Repos.Queue().Update(ctx, queue)
}

// NewUninstallQueueTask creates an uninstall queue job
func NewUninstallQueueTask(siteID, queueID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.Task(TypeUninstallQueue, UninstallQueuePayload{
		SiteID:  siteID,
		QueueID: queueID,
		UserID:  userID,
	})
}
