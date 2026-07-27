package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	"github.com/kkz6/launch-go/internal/modules/site/tasks"
	sitetypes "github.com/kkz6/launch-go/internal/modules/site/types"
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
	Deps    *JobDeps
	Payload RollbackPayload

	// Model fields for Failed() callback
	site       *models.Site
	server     *servermodels.Server
	deployment *models.Deployment
}

func NewRollbackJob(p RollbackPayload) pkgjobs.Handler {
	return &RollbackJob{Deps: deps, Payload: p}
}

// Handle executes the rollback job
func (j *RollbackJob) Handle(ctx context.Context) error {
	var err error

	// Get current deployment (the new deployment record for this rollback)
	j.deployment, err = j.Deps.Repos.Deployment().FindByID(ctx, j.Payload.DeploymentID)
	if err != nil {
		return fmt.Errorf("failed to find current deployment: %w", err)
	}

	// Get target deployment (the deployment to roll back to)
	targetDeployment, err := j.Deps.Repos.Deployment().FindByID(ctx, j.Payload.TargetDeploymentID)
	if err != nil {
		return fmt.Errorf("failed to find target deployment: %w", err)
	}

	// Get site
	j.site, err = j.Deps.Repos.Site().FindByID(ctx, j.Payload.SiteID)
	if err != nil {
		return fmt.Errorf("failed to find site: %w", err)
	}

	// Get server
	j.server, err = j.Deps.ServerRepos.Server().FindByID(ctx, j.site.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	// Update deployment status to Installing
	j.deployment.Status = sitetypes.DeploymentStatusInstalling
	if err := j.Deps.Repos.Deployment().Update(ctx, j.deployment); err != nil {
		j.Deps.Logger.Error().Err(err).Msg("Failed to update deployment status to installing")
	}

	// Broadcast rollback started
	j.Deps.BroadcastServerEvent(j.server, "deployment.rollback.started", map[string]interface{}{
		"team_id":              j.server.TeamID,
		"site_id":              j.site.ID,
		"deployment_id":        j.deployment.ID,
		"target_deployment_id": targetDeployment.ID,
	})

	// Calculate release directory from target deployment's created_at timestamp
	releaseDirectory := fmt.Sprintf("%s/releases/%d", j.site.Path, targetDeployment.CreatedAt.Unix())
	currentDirectory := fmt.Sprintf("%s/current", j.site.Path)

	// Create rollback task
	task := tasks.RollbackDeployment(tasks.RollbackDeploymentConfig{
		SitePath:         j.site.Path,
		ReleaseDirectory: releaseDirectory,
		CurrentDirectory: currentDirectory,
	})

	j.Deps.Logger.Info().
		Str("site_id", j.site.ID).
		Str("deployment_id", j.deployment.ID).
		Str("target_deployment_id", targetDeployment.ID).
		Str("release_directory", releaseDirectory).
		Msg("Executing rollback")

	// Execute the task on the server as site user (with DB tracking)
	result, err := j.Deps.RunTask(j.server, task).AsUser(j.site.User).TrackInDB().Dispatch(ctx)
	if err != nil {
		j.handleRollbackFailure(ctx, j.deployment, j.site.ID, targetDeployment.ID, err)
		return err
	}

	// Update deployment with task ID
	if result.TaskModel != nil {
		j.deployment.TaskID = &result.TaskModel.ID
		if updateErr := j.Deps.Repos.Deployment().Update(ctx, j.deployment); updateErr != nil {
			j.Deps.Logger.Error().Err(updateErr).Msg("Failed to update deployment with task ID")
		}
	}

	exitCode := result.GetExitCode()

	if exitCode != 0 {
		j.handleRollbackFailure(ctx, j.deployment, j.site.ID, targetDeployment.ID,
			fmt.Errorf("rollback failed with exit code %d", exitCode))
		return fmt.Errorf("rollback failed with exit code %d", exitCode)
	}

	// Success - update deployment status
	j.handleRollbackSuccess(ctx, j.deployment, j.site.ID, targetDeployment.ID)

	return nil
}

// handleRollbackSuccess handles successful rollback
func (j *RollbackJob) handleRollbackSuccess(ctx context.Context, deployment *models.Deployment, siteID, targetDeploymentID string) {
	deployment.Status = sitetypes.DeploymentStatusFinished

	if err := j.Deps.Repos.Deployment().Update(ctx, deployment); err != nil {
		j.Deps.Logger.Error().Err(err).Msg("Failed to update deployment status to finished")
	}

	// Get site and server for broadcasting
	site, err := j.Deps.Repos.Site().FindByID(ctx, siteID)
	if err != nil {
		j.Deps.Logger.Error().Err(err).Msg("Failed to find site for broadcast")
		return
	}

	server, err := j.Deps.ServerRepos.Server().FindByID(ctx, site.ServerID)
	if err != nil {
		j.Deps.Logger.Error().Err(err).Msg("Failed to find server for broadcast")
		return
	}

	// Broadcast rollback completed
	j.Deps.BroadcastServerEvent(server, "deployment.rollback.completed", map[string]interface{}{
		"team_id":              server.TeamID,
		"site_id":              siteID,
		"deployment_id":        deployment.ID,
		"target_deployment_id": targetDeploymentID,
	})

	j.Deps.Logger.Info().
		Str("site_id", siteID).
		Str("deployment_id", deployment.ID).
		Str("target_deployment_id", targetDeploymentID).
		Msg("Rollback completed successfully")
}

// handleRollbackFailure handles failed rollback
func (j *RollbackJob) handleRollbackFailure(ctx context.Context, deployment *models.Deployment, siteID, targetDeploymentID string, err error) {
	deployment.Status = sitetypes.DeploymentStatusFailed

	if updateErr := j.Deps.Repos.Deployment().Update(ctx, deployment); updateErr != nil {
		j.Deps.Logger.Error().Err(updateErr).Msg("Failed to update deployment status to failed")
	}

	// Get site and server for broadcasting
	site, siteErr := j.Deps.Repos.Site().FindByID(ctx, siteID)
	if siteErr != nil {
		j.Deps.Logger.Error().Err(siteErr).Msg("Failed to find site for broadcast")
		j.Deps.Logger.Error().Err(err).
			Str("site_id", siteID).
			Str("deployment_id", deployment.ID).
			Str("target_deployment_id", targetDeploymentID).
			Msg("Rollback failed")
		return
	}

	server, serverErr := j.Deps.ServerRepos.Server().FindByID(ctx, site.ServerID)
	if serverErr != nil {
		j.Deps.Logger.Error().Err(serverErr).Msg("Failed to find server for broadcast")
		j.Deps.Logger.Error().Err(err).
			Str("site_id", siteID).
			Str("deployment_id", deployment.ID).
			Str("target_deployment_id", targetDeploymentID).
			Msg("Rollback failed")
		return
	}

	// Broadcast rollback failed
	j.Deps.BroadcastServerEvent(server, "deployment.rollback.failed", map[string]interface{}{
		"team_id":              server.TeamID,
		"site_id":              siteID,
		"deployment_id":        deployment.ID,
		"target_deployment_id": targetDeploymentID,
		"error":                err.Error(),
	})

	j.Deps.Logger.Error().Err(err).
		Str("site_id", siteID).
		Str("deployment_id", deployment.ID).
		Str("target_deployment_id", targetDeploymentID).
		Msg("Rollback failed")
}

// Failed handles job failure
func (j *RollbackJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).
		Str("site_id", j.Payload.SiteID).
		Str("deployment_id", j.Payload.DeploymentID).
		Msg("Rollback job failed")

	// Try to update deployment status
	deployment, findErr := j.Deps.Repos.Deployment().FindByID(ctx, j.Payload.DeploymentID)
	if findErr != nil {
		return
	}

	deployment.Status = sitetypes.DeploymentStatusFailed
	if updateErr := j.Deps.Repos.Deployment().Update(ctx, deployment); updateErr != nil {
		j.Deps.Logger.Error().Err(updateErr).Str("deployment_id", deployment.ID).Msg("Failed to mark rollback deployment failed")
	}

	// Get site and server for broadcasting
	site, siteErr := j.Deps.Repos.Site().FindByID(ctx, j.Payload.SiteID)
	if siteErr != nil {
		return
	}

	server, serverErr := j.Deps.ServerRepos.Server().FindByID(ctx, site.ServerID)
	if serverErr != nil {
		return
	}

	// Broadcast failure
	j.Deps.BroadcastServerEvent(server, "deployment.rollback.failed", map[string]interface{}{
		"team_id":              server.TeamID,
		"site_id":              j.Payload.SiteID,
		"deployment_id":        j.Payload.DeploymentID,
		"target_deployment_id": j.Payload.TargetDeploymentID,
		"error":                err.Error(),
	})
}

// NewRollbackTask creates a rollback job
// Uses TaskID for deduplication to prevent duplicate rollbacks
func NewRollbackTask(siteID, deploymentID, targetDeploymentID, userID string) (*asynq.Task, error) {
	var userIDPtr *string
	if userID != "" {
		userIDPtr = &userID
	}
	return pkgjobs.Task(TypeRollback, RollbackPayload{
		SiteID:             siteID,
		DeploymentID:       deploymentID,
		TargetDeploymentID: targetDeploymentID,
		UserID:             userIDPtr,
	}, asynq.TaskID(pkgjobs.Dedup("rollback", deploymentID)))
}
