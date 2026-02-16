package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/hibiken/asynq"

	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	servertasks "github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	"github.com/kkz6/launch-go/internal/modules/site/tasks"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeInstallQueue = "site:install_queue"

// InstallQueuePayload holds data for queue worker installation
type InstallQueuePayload struct {
	SiteID  string  `json:"site_id"`
	QueueID string  `json:"queue_id"`
	UserID  *string `json:"user_id,omitempty"`
}

// InstallQueueJob handles queue worker installation
type InstallQueueJob struct {
	Deps    *JobDeps
	Payload InstallQueuePayload

	// Model fields for Failed() callback
	site   *models.Site
	server *servermodels.Server
	queue  *models.Queue
}

// NewInstallQueueJob creates a new InstallQueueJob with the given payload
func NewInstallQueueJob(p InstallQueuePayload) pkgjobs.Handler {
	return &InstallQueueJob{Deps: deps, Payload: p}
}

// Handle executes the install queue job
func (j *InstallQueueJob) Handle(ctx context.Context) error {
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
		Str("command", queue.Command).
		Msg("Installing queue worker")

	// Get server username
	serverUsername := "launch"
	if server.Username != nil {
		serverUsername = *server.Username
	}

	// Build supervisor config
	configContent := tasks.BuildQueueSupervisorConfig(queue, serverUsername)
	configPath := queue.GetPath()

	// Upload the config
	uploadTask := tasks.UploadQueueConfig(tasks.UploadQueueConfigParams{
		Path:         configPath,
		Contents:     configContent,
		LogPath:      queue.GetLogPath(),
		ErrorLogPath: queue.GetErrorLogPath(),
		User:         queue.User,
	})

	result, err := j.Deps.RunTask(server, uploadTask).AsUser().Dispatch(ctx)
	if err != nil {
		j.Deps.Logger.Error().Err(err).Str("queue_id", queue.ID).Msg("Failed to upload queue config")
		return err
	}

	if result.GetExitCode() != 0 {
		return fmt.Errorf("failed to upload queue config: exit code %d", result.GetExitCode())
	}

	// Reload supervisor
	reloadTask := servertasks.ReloadSupervisor()
	result, err = j.Deps.RunTask(server, reloadTask).AsUser().Dispatch(ctx)
	if err != nil {
		j.Deps.Logger.Error().Err(err).Str("queue_id", queue.ID).Msg("Failed to reload supervisor")
		return err
	}

	if result.GetExitCode() != 0 {
		j.Deps.Logger.Error().Str("queue_id", queue.ID).Int("exit_code", result.GetExitCode()).Msg("Supervisor reload failed")
	}

	// Mark queue as installed
	now := time.Now()
	queue.InstalledAt = &now
	queue.InstallationFailedAt = nil
	if err := j.Deps.Repos.Queue().Update(ctx, queue); err != nil {
		j.Deps.Logger.Error().Err(err).Msg("Failed to update queue installed status")
	}

	// Broadcast success
	j.Deps.BroadcastServerEvent(server, "queue.installed", map[string]interface{}{
		"site_id":  site.ID,
		"queue_id": queue.ID,
	})

	j.Deps.Logger.Info().Str("queue_id", queue.ID).Msg("Queue worker installed successfully")

	return nil
}

// Failed handles job failure
func (j *InstallQueueJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).
		Str("site_id", j.Payload.SiteID).
		Str("queue_id", j.Payload.QueueID).
		Msg("Install queue job failed")

	// Mark installation as failed
	queue, findErr := j.Deps.Repos.Queue().FindByID(ctx, j.Payload.QueueID)
	if findErr != nil {
		return
	}

	now := time.Now()
	queue.InstalledAt = nil
	queue.InstallationFailedAt = &now
	_ = j.Deps.Repos.Queue().Update(ctx, queue)
}

// NewInstallQueueTask creates an install queue job
func NewInstallQueueTask(siteID, queueID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.Task(TypeInstallQueue, InstallQueuePayload{
		SiteID:  siteID,
		QueueID: queueID,
		UserID:  userID,
	})
}
