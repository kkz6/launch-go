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
	Deps    *JobDeps
	Payload CreateDeploymentPayload

	// Model fields for Failed() callback
	site *models.Site
}

// NewCreateDeploymentJob creates a new CreateDeploymentJob
func NewCreateDeploymentJob(p CreateDeploymentPayload) pkgjobs.Handler {
	return &CreateDeploymentJob{Deps: deps, Payload: p}
}

// Handle executes the create deployment job
func (j *CreateDeploymentJob) Handle(ctx context.Context) error {
	site, err := j.Deps.Repos.Site().FindByID(ctx, j.Payload.SiteID)
	if err != nil {
		return fmt.Errorf("failed to find site: %w", err)
	}
	j.site = site

	j.Deps.Logger.Info().Str("site_id", site.ID).Msg("Creating deployment")

	// Check if there's already an active deployment
	activeDeployment, err := j.Deps.Repos.Deployment().FindActiveBySite(ctx, site.ID)
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

	if err := j.Deps.Repos.Deployment().Create(ctx, deployment); err != nil {
		return fmt.Errorf("failed to create deployment: %w", err)
	}

	j.Deps.Logger.Info().
		Str("site_id", site.ID).
		Str("deployment_id", deployment.ID).
		Str("status", string(status)).
		Msg("Deployment created")

	// If queued, we're done - it will be processed when the active deployment finishes
	if status == sitetypes.DeploymentStatusQueued {
		j.Deps.Logger.Info().Str("deployment_id", deployment.ID).Msg("Deployment queued behind active deployment")
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
	if err := j.Deps.DispatchTaskIn(task, time.Second); err != nil {
		// Cleanup: mark deployment as failed if we can't dispatch the job
		_ = j.Deps.Repos.Deployment().UpdateStatus(ctx, deployment.ID, sitetypes.DeploymentStatusFailed)
		return fmt.Errorf("failed to enqueue deploy job: %w", err)
	}

	j.Deps.Logger.Info().
		Str("site_id", site.ID).
		Str("deployment_id", deployment.ID).
		Bool("zero_downtime", site.ZeroDowntimeDeployment).
		Msg("Deploy job dispatched")

	return nil
}

// Failed handles job failure
func (j *CreateDeploymentJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).Str("site_id", j.Payload.SiteID).Msg("Failed to create deployment")
}

// NewCreateDeploymentTask creates a create deployment task
func NewCreateDeploymentTask(siteID string, userID *string, gitHash *string, branch *string) (*asynq.Task, error) {
	return pkgjobs.Task(TypeCreateDeployment, CreateDeploymentPayload{
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
