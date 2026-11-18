package jobs

import (
	"context"
	"errors"
	"fmt"
	gitaccess "github.com/kkz6/launch-go/internal/modules/git/access"
	siteaccess "github.com/kkz6/launch-go/internal/modules/site/gitaccess"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/site/deployments"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	sitetypes "github.com/kkz6/launch-go/internal/modules/site/types"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeCreateDeployment = "site:create_deployment"

// CreateDeploymentPayload holds data for creating a deployment
type CreateDeploymentPayload struct {
	SiteID     string         `json:"site_id"`
	UserID     *string        `json:"user_id,omitempty"`
	GitHash    *string        `json:"git_hash,omitempty"`
	Branch     *string        `json:"branch,omitempty"`
	CommitData map[string]any `json:"commit_data,omitempty"`
	GitAccess  dbtype.JSONMap `json:"git_access,omitempty"`
	WebhookKey string         `json:"webhook_key,omitempty"`
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

	deployment := &models.Deployment{}
	existing := false
	err = j.Deps.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Serialize this site's intent creation, including queue redelivery.
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(site, "id = ?", site.ID).Error; err != nil {
			return err
		}
		if j.Payload.Branch != nil && *j.Payload.Branch != site.GetRepositoryBranch() {
			return fmt.Errorf("deployment branch changed")
		}
		if j.Payload.WebhookKey != "" {
			findErr := tx.Where("webhook_key = ?", j.Payload.WebhookKey).First(deployment).Error
			if findErr == nil {
				if deployment.SiteID != site.ID {
					return fmt.Errorf("deployment intent mismatch")
				}
				existing = true
				_, err := siteaccess.ForDeployment(ctx, tx, j.Deps.ProviderFactory, site, deployment, gitaccess.Clone)
				return err
			}
			if !errors.Is(findErr, gorm.ErrRecordNotFound) {
				return findErr
			}
		}
		deployment.SiteID, deployment.TeamID = site.ID, site.TeamID
		deployment.GitAccess = j.Payload.GitAccess
		if _, err := siteaccess.ForDeployment(ctx, tx, j.Deps.ProviderFactory, site, deployment, gitaccess.Clone); err != nil {
			return err
		}
		var active int64
		if err := tx.Model(&models.Deployment{}).Where("site_id = ? AND status IN ?", site.ID, []sitetypes.DeploymentStatus{sitetypes.DeploymentStatusPending, sitetypes.DeploymentStatusInstalling}).Count(&active).Error; err != nil {
			return err
		}
		deployment.Status = sitetypes.DeploymentStatusPending
		if active > 0 {
			if !site.QueueDeployments {
				return fmt.Errorf("deployment already in progress")
			}
			deployment.Status = sitetypes.DeploymentStatusQueued
		}
		deployment.GitHash = j.Payload.GitHash
		if deployment.GitHash == nil {
			if sha, ok := j.Payload.CommitData["sha"].(string); ok && sha != "" {
				deployment.GitHash = &sha
			}
		}
		deployment.CommitData = j.Payload.CommitData
		deployment.UserID = j.Payload.UserID
		if j.Payload.WebhookKey != "" {
			deployment.WebhookKey = &j.Payload.WebhookKey
		}
		return tx.Create(deployment).Error
	})
	if err != nil {
		return err
	}
	if existing && deployment.Status != sitetypes.DeploymentStatusPending {
		return nil
	}
	status := deployment.Status

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

	// A durable webhook intent retains its pending row during a queue outage.
	// Redelivery uses the same row and task ID; it never creates another deploy.
	if j.Payload.WebhookKey != "" {
		task, err := deployments.NewTask(site.ID, deployment.ID, stringPtrValue(j.Payload.UserID), nil, site.ZeroDowntimeDeployment)
		if err != nil {
			return fmt.Errorf("failed to create deploy task: %w", err)
		}
		if j.Deps.Queue == nil {
			return fmt.Errorf("deployment queue unavailable")
		}
		_, err = j.Deps.Queue.Enqueue(task, asynq.ProcessIn(time.Second))
		if errors.Is(err, asynq.ErrTaskIDConflict) || errors.Is(err, asynq.ErrDuplicateTask) {
			return nil
		}
		return err
	}

	// Add a small delay to ensure DB transaction is committed
	if err := j.Deps.DeploymentDispatcher().Dispatch(ctx, deployment, func() (*asynq.Task, error) {
		return deployments.NewTask(site.ID, deployment.ID, stringPtrValue(j.Payload.UserID), nil, site.ZeroDowntimeDeployment)
	}, asynq.ProcessIn(time.Second)); err != nil {
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
func NewCreateDeploymentTask(siteID string, userID *string, gitHash *string, branch *string, commitData map[string]any) (*asynq.Task, error) {
	return pkgjobs.Task(TypeCreateDeployment, CreateDeploymentPayload{
		SiteID:     siteID,
		UserID:     userID,
		GitHash:    gitHash,
		Branch:     branch,
		CommitData: commitData,
	})
}

func stringPtrValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
