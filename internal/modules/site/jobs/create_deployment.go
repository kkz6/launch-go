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
	pkgjobs.BaseJob[*JobContext, CreateDeploymentPayload]
}

// NewCreateDeploymentJob creates a new CreateDeploymentJob
func NewCreateDeploymentJob(ctx *JobContext, payload CreateDeploymentPayload) *CreateDeploymentJob {
	return &CreateDeploymentJob{
		BaseJob: pkgjobs.NewBaseJob(ctx, payload),
	}
}

// Handle executes the create deployment job
func (j *CreateDeploymentJob) Handle(ctx context.Context) error {
	site, err := j.Ctx.SiteRepo.FindByID(ctx, j.Payload.SiteID)
	if err != nil {
		return fmt.Errorf("failed to find site: %w", err)
	}

	j.Ctx.LogInfo("Creating deployment",
		"site_id", site.ID,
	)

	// Check if there's already an active deployment
	activeDeployment, err := j.Ctx.DeploymentRepo.FindActiveBySite(ctx, site.ID)
	if err != nil {
		return fmt.Errorf("failed to check for active deployment: %w", err)
	}

	// Determine initial status
	status := sitetypes.DeploymentStatusPending
	if activeDeployment != nil && site.QueueDeployments {
		// Queue this deployment if there's already an active one
		status = sitetypes.DeploymentStatusQueued
	} else if activeDeployment != nil {
		// Can't start a new deployment while one is active and queuing is disabled
		return fmt.Errorf("deployment already in progress")
	}

	// Create deployment record
	deployment := &models.Deployment{
		Status:  status,
		GitHash: j.Payload.GitHash,
	}
	deployment.SiteID = site.ID

	if err := j.Ctx.DeploymentRepo.Create(ctx, deployment); err != nil {
		return fmt.Errorf("failed to create deployment: %w", err)
	}

	j.Ctx.LogInfo("Deployment created",
		"site_id", site.ID,
		"deployment_id", deployment.ID,
		"status", status,
	)

	// If queued, we're done - it will be processed when the active deployment finishes
	if status == sitetypes.DeploymentStatusQueued {
		j.Ctx.LogInfo("Deployment queued behind active deployment",
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

	// Add a small delay to ensure DB transaction is committed
	if err := j.Ctx.DispatchTaskIn(task, time.Second); err != nil {
		// Cleanup: mark deployment as failed if we can't dispatch the job
		_ = j.Ctx.DeploymentRepo.UpdateStatus(ctx, deployment.ID, sitetypes.DeploymentStatusFailed)
		return fmt.Errorf("failed to enqueue deploy job: %w", err)
	}

	j.Ctx.LogInfo("Deploy job dispatched",
		"site_id", site.ID,
		"deployment_id", deployment.ID,
		"zero_downtime", site.ZeroDowntimeDeployment,
	)

	return nil
}

// Failed handles job failure
func (j *CreateDeploymentJob) Failed(ctx context.Context, err error) {
	j.Ctx.LogError(err, "Failed to create deployment",
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
