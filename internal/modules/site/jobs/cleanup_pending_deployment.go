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

const TypeCleanupPendingSiteDeployment = "site:cleanup_pending_deployment"

// CleanupPendingSiteDeploymentPayload holds data for cleanup
type CleanupPendingSiteDeploymentPayload struct {
	SiteID       string  `json:"site_id"`
	DeploymentID string  `json:"deployment_id"`
	UserID       *string `json:"user_id,omitempty"`
}

// CleanupPendingSiteDeploymentJob cleans up deployments stuck in pending/installing state
type CleanupPendingSiteDeploymentJob struct {
	ctx     *JobContext
	Payload CleanupPendingSiteDeploymentPayload
}

// NewCleanupPendingSiteDeploymentJob creates a new CleanupPendingSiteDeploymentJob
func NewCleanupPendingSiteDeploymentJob(ctx *JobContext, payload CleanupPendingSiteDeploymentPayload) *CleanupPendingSiteDeploymentJob {
	return &CleanupPendingSiteDeploymentJob{
		ctx:     ctx,
		Payload: payload,
	}
}

// Handle executes the cleanup job
func (j *CleanupPendingSiteDeploymentJob) Handle(ctx context.Context) error {
	deployment, err := j.ctx.DeploymentRepo.FindByID(ctx, j.Payload.DeploymentID)
	if err != nil {
		return fmt.Errorf("failed to find deployment: %w", err)
	}

	// Only cleanup if still in pending or installing status
	if !deployment.Status.IsActive() {
		j.ctx.LogInfo("Deployment no longer active, skipping cleanup",
			"deployment_id", deployment.ID,
			"status", deployment.Status,
		)
		return nil
	}

	site, err := j.ctx.SiteRepo.FindByID(ctx, j.Payload.SiteID)
	if err != nil {
		return fmt.Errorf("failed to find site: %w", err)
	}

	j.ctx.LogInfo("Cleaning up stuck deployment",
		"site_id", site.ID,
		"deployment_id", deployment.ID,
		"status", deployment.Status,
	)

	// Mark deployment as timed out
	deployment.Status = enums.DeploymentStatusTimeout
	if err := j.ctx.DeploymentRepo.Update(ctx, deployment); err != nil {
		return fmt.Errorf("failed to update deployment status: %w", err)
	}

	// If this was the first deployment, mark site installation as failed
	if site.InstalledAt == nil {
		site.MarkAsFailed()
		if err := j.ctx.SiteRepo.Update(ctx, site); err != nil {
			j.ctx.LogError(err, "Failed to mark site installation as failed")
		}
	}

	// Broadcast cleanup event
	server, err := j.ctx.ServerRepos.Server().FindByID(ctx, site.ServerID)
	if err == nil && server != nil {
		j.ctx.BroadcastServerEvent(server, "deployment.timeout", map[string]interface{}{
			"team_id":       server.TeamID,
			"site_id":       site.ID,
			"deployment_id": deployment.ID,
			"message":       "Deployment timed out and was cleaned up",
		})
	}

	j.ctx.LogInfo("Stuck deployment cleaned up",
		"site_id", site.ID,
		"deployment_id", deployment.ID,
	)

	// Process next queued deployment if any
	j.processNextQueuedDeployment(ctx, site)

	return nil
}

// processNextQueuedDeployment processes the next queued deployment for the site
func (j *CleanupPendingSiteDeploymentJob) processNextQueuedDeployment(ctx context.Context, site *models.Site) {
	queuedDeployments, err := j.ctx.DeploymentRepo.FindQueuedBySite(ctx, site.ID)
	if err != nil || len(queuedDeployments) == 0 {
		return
	}

	nextDeployment := &queuedDeployments[0]
	nextDeployment.Status = enums.DeploymentStatusPending

	if err := j.ctx.DeploymentRepo.Update(ctx, nextDeployment); err != nil {
		j.ctx.LogError(err, "Failed to update queued deployment status")
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
		j.ctx.LogError(taskErr, "Failed to create deployment task for queued deployment")
		return
	}

	if j.ctx.Queue != nil {
		if _, err := j.ctx.Queue.Enqueue(task); err != nil {
			j.ctx.LogError(err, "Failed to enqueue next deployment")
		}
	}
}

// Failed handles job failure
func (j *CleanupPendingSiteDeploymentJob) Failed(ctx context.Context, err error) {
	j.ctx.LogError(err, "Failed to cleanup pending deployment",
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
