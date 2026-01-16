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

const TypeCreateDeployment = "site:create_deployment"

// CreateDeploymentPayload holds data for creating a deployment
type CreateDeploymentPayload struct {
	SiteID  string  `json:"site_id"`
	UserID  *string `json:"user_id,omitempty"`
	GitHash *string `json:"git_hash,omitempty"`
	Branch  *string `json:"branch,omitempty"`
}

// CreateDeploymentJob creates a deployment record and dispatches the deploy job
type CreateDeploymentJob struct {
	ctx     *JobContext
	Payload CreateDeploymentPayload
}

// NewCreateDeploymentJob creates a new CreateDeploymentJob
func NewCreateDeploymentJob(ctx *JobContext, payload CreateDeploymentPayload) *CreateDeploymentJob {
	return &CreateDeploymentJob{
		ctx:     ctx,
		Payload: payload,
	}
}

// Handle executes the create deployment job
func (j *CreateDeploymentJob) Handle(ctx context.Context) error {
	site, err := j.ctx.SiteRepo.FindByID(ctx, j.Payload.SiteID)
	if err != nil {
		return fmt.Errorf("failed to find site: %w", err)
	}

	j.ctx.LogInfo("Creating deployment",
		"site_id", site.ID,
	)

	// Check if there's already an active deployment
	activeDeployment, err := j.ctx.DeploymentRepo.FindActiveBySite(ctx, site.ID)
	if err != nil {
		return fmt.Errorf("failed to check for active deployment: %w", err)
	}

	// Determine initial status
	status := enums.DeploymentStatusPending
	if activeDeployment != nil && site.QueueDeployments {
		// Queue this deployment if there's already an active one
		status = enums.DeploymentStatusQueued
	} else if activeDeployment != nil {
		// Can't start a new deployment while one is active and queuing is disabled
		return fmt.Errorf("deployment already in progress")
	}

	// Create deployment record
	deployment := &models.Deployment{
		SiteID:  site.ID,
		Status:  status,
		GitHash: j.Payload.GitHash,
	}

	if err := j.ctx.DeploymentRepo.Create(ctx, deployment); err != nil {
		return fmt.Errorf("failed to create deployment: %w", err)
	}

	j.ctx.LogInfo("Deployment created",
		"site_id", site.ID,
		"deployment_id", deployment.ID,
		"status", status,
	)

	// If queued, we're done - it will be processed when the active deployment finishes
	if status == enums.DeploymentStatusQueued {
		j.ctx.LogInfo("Deployment queued behind active deployment",
			"deployment_id", deployment.ID,
		)
		return nil
	}

	// Dispatch the appropriate deploy job
	var task *asynq.Task
	var taskErr error

	if site.ZeroDowntimeDeployment {
		task, taskErr = NewDeployZeroDowntimeTask(site.ID, deployment.ID, stringPtrValue(j.Payload.UserID))
	} else {
		task, taskErr = NewDeployTask(site.ID, deployment.ID, stringPtrValue(j.Payload.UserID))
	}

	if taskErr != nil {
		return fmt.Errorf("failed to create deploy task: %w", taskErr)
	}

	if j.ctx.Queue != nil {
		// Add a small delay to ensure DB transaction is committed
		if _, err := j.ctx.Queue.Enqueue(task, asynq.ProcessIn(time.Second)); err != nil {
			// Cleanup: mark deployment as failed if we can't dispatch the job
			_ = j.ctx.DeploymentRepo.UpdateStatus(ctx, deployment.ID, enums.DeploymentStatusFailed)
			return fmt.Errorf("failed to enqueue deploy job: %w", err)
		}
	}

	j.ctx.LogInfo("Deploy job dispatched",
		"site_id", site.ID,
		"deployment_id", deployment.ID,
		"zero_downtime", site.ZeroDowntimeDeployment,
	)

	return nil
}

// Failed handles job failure
func (j *CreateDeploymentJob) Failed(ctx context.Context, err error) {
	j.ctx.LogError(err, "Failed to create deployment",
		"site_id", j.Payload.SiteID,
	)
}

// NewCreateDeploymentTask creates a create deployment task
func NewCreateDeploymentTask(siteID string, userID *string, gitHash *string, branch *string) (*asynq.Task, error) {
	return pkgjobs.NewTask(TypeCreateDeployment, CreateDeploymentPayload{
		SiteID:  siteID,
		UserID:  userID,
		GitHash: gitHash,
		Branch:  branch,
	})
}

func stringPtrValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
