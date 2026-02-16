package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/hibiken/asynq"

	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	sitetypes "github.com/kkz6/launch-go/internal/modules/site/types"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeCleanupPendingSiteDeployment = "site:cleanup_pending_deployment"

// CleanupPendingSiteDeploymentPayload holds data for cleanup
type CleanupPendingSiteDeploymentPayload struct {
	SiteID       string  `json:"site_id"`
	DeploymentID string  `json:"deployment_id"`
	UserID       *string `json:"user_id,omitempty"`
}

// CleanupPendingSiteDeploymentJob cleans up deployments stuck in pending/installing state
type CleanupPendingSiteDeploymentJob struct {
	Deps    *JobDeps
	Payload CleanupPendingSiteDeploymentPayload

	// Model fields for Failed() callback
	site       *models.Site
	server     *servermodels.Server
	deployment *models.Deployment
}

// NewCleanupPendingSiteDeploymentJob creates a new CleanupPendingSiteDeploymentJob
func NewCleanupPendingSiteDeploymentJob(p CleanupPendingSiteDeploymentPayload) pkgjobs.Handler {
	return &CleanupPendingSiteDeploymentJob{Deps: deps, Payload: p}
}

// Handle executes the cleanup job
func (j *CleanupPendingSiteDeploymentJob) Handle(ctx context.Context) error {
	deployment, err := j.Deps.Repos.Deployment().FindByID(ctx, j.Payload.DeploymentID)
	if err != nil {
		return fmt.Errorf("failed to find deployment: %w", err)
	}
	j.deployment = deployment

	// Only cleanup if still in pending or installing status
	if !deployment.Status.IsActive() {
		j.Deps.Logger.Info().
			Str("deployment_id", deployment.ID).
			Str("status", string(deployment.Status)).
			Msg("Deployment no longer active, skipping cleanup")
		return nil
	}

	site, err := j.Deps.Repos.Site().FindByID(ctx, j.Payload.SiteID)
	if err != nil {
		return fmt.Errorf("failed to find site: %w", err)
	}
	j.site = site

	j.Deps.Logger.Info().
		Str("site_id", site.ID).
		Str("deployment_id", deployment.ID).
		Str("status", string(deployment.Status)).
		Msg("Cleaning up stuck deployment")

	// Mark deployment as timed out
	deployment.Status = sitetypes.DeploymentStatusTimeout
	if err := j.Deps.Repos.Deployment().Update(ctx, deployment); err != nil {
		return fmt.Errorf("failed to update deployment status: %w", err)
	}

	// If this was the first deployment, mark site installation as failed
	if site.InstalledAt == nil {
		if err := j.Deps.Repos.Site().MarkAsFailed(ctx, site.ID); err != nil {
			j.Deps.Logger.Error().Err(err).Msg("Failed to mark site installation as failed")
		}
	}

	// Broadcast cleanup event
	server, err := j.Deps.ServerRepos.Server().FindByID(ctx, site.ServerID)
	if err == nil && server != nil {
		j.server = server
		j.Deps.BroadcastServerEvent(server, "deployment.timeout", map[string]interface{}{
			"team_id":       server.TeamID,
			"site_id":       site.ID,
			"deployment_id": deployment.ID,
			"status":        "timeout",
			"message":       "Deployment timed out and was cleaned up",
		})

		// Also broadcast as deployment.progress so existing listeners pick up the terminal status
		j.Deps.BroadcastServerEvent(server, "deployment.progress", map[string]interface{}{
			"team_id":       server.TeamID,
			"site_id":       site.ID,
			"deployment_id": deployment.ID,
			"status":        "timeout",
			"message":       "Deployment timed out and was cleaned up",
		})
	}

	j.Deps.Logger.Info().Str("site_id", site.ID).Str("deployment_id", deployment.ID).Msg("Stuck deployment cleaned up")

	// Process next queued deployment if any
	j.processNextQueuedDeployment(ctx, site)

	return nil
}

// processNextQueuedDeployment processes the next queued deployment for the site
func (j *CleanupPendingSiteDeploymentJob) processNextQueuedDeployment(ctx context.Context, site *models.Site) {
	queuedDeployments, err := j.Deps.Repos.Deployment().FindQueuedBySite(ctx, site.ID)
	if err != nil || len(queuedDeployments) == 0 {
		return
	}

	nextDeployment := &queuedDeployments[0]
	nextDeployment.Status = sitetypes.DeploymentStatusPending

	if err := j.Deps.Repos.Deployment().Update(ctx, nextDeployment); err != nil {
		j.Deps.Logger.Error().Err(err).Msg("Failed to update queued deployment status")
		return
	}

	// Dispatch next deployment
	var task *asynq.Task
	var taskErr error

	if site.ZeroDowntimeDeployment {
		task, taskErr = NewDeployZeroDowntimeTask(site.ID, nextDeployment.ID, "", nil)
	} else {
		task, taskErr = NewDeployTask(site.ID, nextDeployment.ID, "", nil)
	}

	if taskErr != nil {
		j.Deps.Logger.Error().Err(taskErr).Msg("Failed to create deployment task for queued deployment")
		return
	}

	if err := j.Deps.DispatchTask(task); err != nil {
		j.Deps.Logger.Error().Err(err).Msg("Failed to enqueue next deployment")
	}
}

// Failed handles job failure
func (j *CleanupPendingSiteDeploymentJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).
		Str("site_id", j.Payload.SiteID).
		Str("deployment_id", j.Payload.DeploymentID).
		Msg("Failed to cleanup pending deployment")
}

// NewCleanupPendingSiteDeploymentTask creates a cleanup pending deployment task
func NewCleanupPendingSiteDeploymentTask(siteID, deploymentID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.Task(TypeCleanupPendingSiteDeployment, CleanupPendingSiteDeploymentPayload{
		SiteID:       siteID,
		DeploymentID: deploymentID,
		UserID:       userID,
	})
}

// NewCleanupPendingSiteDeploymentTaskDelayed creates a delayed cleanup task
// This is typically dispatched when a deployment starts, to auto-cleanup if it hangs
func NewCleanupPendingSiteDeploymentTaskDelayed(siteID, deploymentID string, userID *string, delay time.Duration) (*asynq.Task, error) {
	task, err := pkgjobs.Task(TypeCleanupPendingSiteDeployment, CleanupPendingSiteDeploymentPayload{
		SiteID:       siteID,
		DeploymentID: deploymentID,
		UserID:       userID,
	})
	if err != nil {
		return nil, err
	}

	// Note: The caller should use asynq.ProcessIn(delay) when enqueueing
	return task, nil
}
