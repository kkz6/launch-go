package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/site/enums"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	"github.com/kkz6/launch-go/internal/modules/site/tasks"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeRollback = "site:rollback"

// RollbackPayload holds data for deployment rollback
type RollbackPayload struct {
	SiteID             string  `json:"site_id"`
	DeploymentID       string  `json:"deployment_id"`
	TargetDeploymentID string  `json:"target_deployment_id"`
	UserID             *string `json:"user_id,omitempty"`
}

// RollbackJob handles deployment rollback
type RollbackJob struct {
	ctx     *JobContext
	Payload RollbackPayload
}

// NewRollbackJob creates a new rollback job
func NewRollbackJob(ctx *JobContext, payload RollbackPayload) *RollbackJob {
	return &RollbackJob{
		ctx:     ctx,
		Payload: payload,
	}
}

// Handle executes the rollback job
func (j *RollbackJob) Handle(ctx context.Context) error {
	// Get current deployment (the new deployment record for this rollback)
	currentDeployment, err := j.ctx.DeploymentRepo.FindByID(ctx, j.Payload.DeploymentID)
	if err != nil {
		return fmt.Errorf("failed to find current deployment: %w", err)
	}

	// Get target deployment (the deployment to roll back to)
	targetDeployment, err := j.ctx.DeploymentRepo.FindByID(ctx, j.Payload.TargetDeploymentID)
	if err != nil {
		return fmt.Errorf("failed to find target deployment: %w", err)
	}

	// Get site
	site, err := j.ctx.SiteRepo.FindByID(ctx, j.Payload.SiteID)
	if err != nil {
		return fmt.Errorf("failed to find site: %w", err)
	}

	// Get server
	server, err := j.ctx.ServerRepo.FindServerByID(ctx, site.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	// Update deployment status to Installing
	currentDeployment.Status = enums.DeploymentStatusInstalling
	now := time.Now()
	currentDeployment.StartedAt = &now
	if err := j.ctx.DeploymentRepo.Update(ctx, currentDeployment); err != nil {
		j.ctx.LogError(err, "Failed to update deployment status to installing")
	}

	// Broadcast rollback started
	j.ctx.BroadcastToSite(site.ID, "deployment.rollback.started", map[string]interface{}{
		"site_id":              site.ID,
		"deployment_id":        currentDeployment.ID,
		"target_deployment_id": targetDeployment.ID,
	})

	// Calculate release directory from target deployment's created_at timestamp
	releaseDirectory := fmt.Sprintf("%s/releases/%d", site.Path, targetDeployment.CreatedAt.Unix())
	currentDirectory := fmt.Sprintf("%s/current", site.Path)

	// Create rollback task
	task := tasks.RollbackDeployment(tasks.RollbackDeploymentConfig{
		SitePath:         site.Path,
		ReleaseDirectory: releaseDirectory,
		CurrentDirectory: currentDirectory,
	})

	j.ctx.LogInfo("Executing rollback",
		"site_id", site.ID,
		"deployment_id", currentDeployment.ID,
		"target_deployment_id", targetDeployment.ID,
		"release_directory", releaseDirectory,
	)

	// Execute the task on the server as site user
	result, err := j.ctx.RunTaskOnServer(server, task).AsUser(site.User).Dispatch(ctx)
	if err != nil {
		j.handleRollbackFailure(ctx, currentDeployment, site.ID, targetDeployment.ID, err)
		return err
	}

	exitCode := result.GetExitCode()
	output := result.GetOutput()

	// Update deployment with output
	currentDeployment.Output = &output

	if exitCode != 0 {
		j.handleRollbackFailure(ctx, currentDeployment, site.ID, targetDeployment.ID,
			fmt.Errorf("rollback failed with exit code %d", exitCode))
		return fmt.Errorf("rollback failed with exit code %d", exitCode)
	}

	// Success - update deployment status
	j.handleRollbackSuccess(ctx, currentDeployment, site.ID, targetDeployment.ID)

	return nil
}

// handleRollbackSuccess handles successful rollback
func (j *RollbackJob) handleRollbackSuccess(ctx context.Context, deployment *models.Deployment, siteID, targetDeploymentID string) {
	now := time.Now()
	deployment.Status = enums.DeploymentStatusFinished
	deployment.FinishedAt = &now

	if err := j.ctx.DeploymentRepo.Update(ctx, deployment); err != nil {
		j.ctx.LogError(err, "Failed to update deployment status to finished")
	}

	// Broadcast rollback completed
	j.ctx.BroadcastToSite(siteID, "deployment.rollback.completed", map[string]interface{}{
		"site_id":              siteID,
		"deployment_id":        deployment.ID,
		"target_deployment_id": targetDeploymentID,
	})

	j.ctx.LogInfo("Rollback completed successfully",
		"site_id", siteID,
		"deployment_id", deployment.ID,
		"target_deployment_id", targetDeploymentID,
	)
}

// handleRollbackFailure handles failed rollback
func (j *RollbackJob) handleRollbackFailure(ctx context.Context, deployment *models.Deployment, siteID, targetDeploymentID string, err error) {
	now := time.Now()
	deployment.Status = enums.DeploymentStatusFailed
	deployment.FinishedAt = &now

	if updateErr := j.ctx.DeploymentRepo.Update(ctx, deployment); updateErr != nil {
		j.ctx.LogError(updateErr, "Failed to update deployment status to failed")
	}

	// Broadcast rollback failed
	j.ctx.BroadcastToSite(siteID, "deployment.rollback.failed", map[string]interface{}{
		"site_id":              siteID,
		"deployment_id":        deployment.ID,
		"target_deployment_id": targetDeploymentID,
		"error":                err.Error(),
	})

	j.ctx.LogError(err, "Rollback failed",
		"site_id", siteID,
		"deployment_id", deployment.ID,
		"target_deployment_id", targetDeploymentID,
	)
}

// Failed handles job failure
func (j *RollbackJob) Failed(ctx context.Context, err error) {
	j.ctx.LogError(err, "Rollback job failed",
		"site_id", j.Payload.SiteID,
		"deployment_id", j.Payload.DeploymentID,
	)

	// Try to update deployment status
	deployment, findErr := j.ctx.DeploymentRepo.FindByID(ctx, j.Payload.DeploymentID)
	if findErr != nil {
		return
	}

	now := time.Now()
	deployment.Status = enums.DeploymentStatusFailed
	deployment.FinishedAt = &now
	_ = j.ctx.DeploymentRepo.Update(ctx, deployment)

	// Broadcast failure
	j.ctx.BroadcastToSite(j.Payload.SiteID, "deployment.rollback.failed", map[string]interface{}{
		"site_id":              j.Payload.SiteID,
		"deployment_id":        j.Payload.DeploymentID,
		"target_deployment_id": j.Payload.TargetDeploymentID,
		"error":                err.Error(),
	})
}

// NewRollbackTask creates a rollback job
func NewRollbackTask(siteID, deploymentID, targetDeploymentID, userID string) (*asynq.Task, error) {
	var userIDPtr *string
	if userID != "" {
		userIDPtr = &userID
	}
	return pkgjobs.NewTask(TypeRollback, RollbackPayload{
		SiteID:             siteID,
		DeploymentID:       deploymentID,
		TargetDeploymentID: targetDeploymentID,
		UserID:             userIDPtr,
	})
}
