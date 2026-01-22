package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/hibiken/asynq"

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
	pkgjobs.BaseJob[*JobContext, CleanupPendingSiteDeploymentPayload]
}

// NewCleanupPendingSiteDeploymentJob creates a new CleanupPendingSiteDeploymentJob
func NewCleanupPendingSiteDeploymentJob(ctx *JobContext, payload CleanupPendingSiteDeploymentPayload) *CleanupPendingSiteDeploymentJob {
	return &CleanupPendingSiteDeploymentJob{
		BaseJob: pkgjobs.NewBaseJob(ctx, payload),
	}
}

// Handle executes the cleanup job
func (j *CleanupPendingSiteDeploymentJob) Handle(ctx context.Context) error {
	deployment, err := j.Ctx.DeploymentRepo.FindByID(ctx, j.Payload.DeploymentID)
	if err != nil {
		return fmt.Errorf("failed to find deployment: %w", err)
	}

	// Only cleanup if still in pending or installing status
	if !deployment.Status.IsActive() {
		j.Ctx.LogInfo("Deployment no longer active, skipping cleanup",
			"deployment_id", deployment.ID,
			"status", deployment.Status,
		)
		return nil
	}

	site, err := j.Ctx.SiteRepo.FindByID(ctx, j.Payload.SiteID)
	if err != nil {
		return fmt.Errorf("failed to find site: %w", err)
	}

	j.Ctx.LogInfo("Cleaning up stuck deployment",
		"site_id", site.ID,
		"deployment_id", deployment.ID,
		"status", deployment.Status,
	)

	// Mark deployment as timed out
	deployment.Status = sitetypes.DeploymentStatusTimeout
	if err := j.Ctx.DeploymentRepo.Update(ctx, deployment); err != nil {
		return fmt.Errorf("failed to update deployment status: %w", err)
	}

	// If this was the first deployment, mark site installation as failed
	if site.InstalledAt == nil {
		if err := j.Ctx.SiteRepo.MarkAsFailed(ctx, site.ID); err != nil {
			j.Ctx.LogError(err, "Failed to mark site installation as failed")
		}
	}

	// Broadcast cleanup event
	server, err := j.Ctx.ServerRepos.Server().FindByID(ctx, site.ServerID)
	if err == nil && server != nil {
		j.Ctx.BroadcastServerEvent(server, "deployment.timeout", map[string]interface{}{
			"team_id":       server.TeamID,
			"site_id":       site.ID,
			"deployment_id": deployment.ID,
			"message":       "Deployment timed out and was cleaned up",
		})
	}

	j.Ctx.LogInfo("Stuck deployment cleaned up",
		"site_id", site.ID,
		"deployment_id", deployment.ID,
	)

	// Process next queued deployment if any
	j.processNextQueuedDeployment(ctx, site)

	return nil
}

// processNextQueuedDeployment processes the next queued deployment for the site
func (j *CleanupPendingSiteDeploymentJob) processNextQueuedDeployment(ctx context.Context, site *models.Site) {
	queuedDeployments, err := j.Ctx.DeploymentRepo.FindQueuedBySite(ctx, site.ID)
	if err != nil || len(queuedDeployments) == 0 {
		return
	}

	nextDeployment := &queuedDeployments[0]
	nextDeployment.Status = sitetypes.DeploymentStatusPending

	if err := j.Ctx.DeploymentRepo.Update(ctx, nextDeployment); err != nil {
		j.Ctx.LogError(err, "Failed to update queued deployment status")
		return
	}

	// Dispatch next deployment
	var task *asynq.Task
	var taskErr error

	if site.ZeroDowntimeDeployment {
		task, taskErr = NewDeployZeroDowntimeTask(site.ID, nextDeployment.ID, "")
	} else {
		task, taskErr = NewDeployTask(site.ID, nextDeployment.ID, "")
	}

	if taskErr != nil {
		j.Ctx.LogError(taskErr, "Failed to create deployment task for queued deployment")
		return
	}

	if err := j.Ctx.DispatchTask(task); err != nil {
		j.Ctx.LogError(err, "Failed to enqueue next deployment")
	}
}

// Failed handles job failure
func (j *CleanupPendingSiteDeploymentJob) Failed(ctx context.Context, err error) {
	j.Ctx.LogError(err, "Failed to cleanup pending deployment",
		"site_id", j.Payload.SiteID,
		"deployment_id", j.Payload.DeploymentID,
	)
}

// NewCleanupPendingSiteDeploymentTask creates a cleanup pending deployment task
func NewCleanupPendingSiteDeploymentTask(siteID, deploymentID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.NewTask(TypeCleanupPendingSiteDeployment, CleanupPendingSiteDeploymentPayload{
		SiteID:       siteID,
		DeploymentID: deploymentID,
		UserID:       userID,
	})
}

// NewCleanupPendingSiteDeploymentTaskDelayed creates a delayed cleanup task
// This is typically dispatched when a deployment starts, to auto-cleanup if it hangs
func NewCleanupPendingSiteDeploymentTaskDelayed(siteID, deploymentID string, userID *string, delay time.Duration) (*asynq.Task, error) {
	task, err := pkgjobs.NewTask(TypeCleanupPendingSiteDeployment, CleanupPendingSiteDeploymentPayload{
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
