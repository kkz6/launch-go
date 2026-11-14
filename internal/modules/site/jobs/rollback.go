package jobs

import (
	"context"
	"errors"
	"fmt"

	"github.com/hibiken/asynq"

	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	siteevents "github.com/kkz6/launch-go/internal/modules/site/events"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	"github.com/kkz6/launch-go/internal/modules/site/tasks"
	sitetypes "github.com/kkz6/launch-go/internal/modules/site/types"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeRollback = "site:rollback"

var (
	errRollbackPostActions    = errors.New("failed to queue post-rollback actions")
	errRollbackAlreadyClaimed = errors.New("rollback was already claimed by another worker")
)

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
func (j *RollbackJob) Handle(ctx context.Context) (handleErr error) {
	completionSaved := false
	// Only event publication is retryable. Replaying a failed or interrupted
	// remote switch could overwrite a newer deployment.
	defer func() {
		if handleErr != nil && completionSaved && !errors.Is(handleErr, errRollbackPostActions) {
			handleErr = fmt.Errorf("%w: %w", errRollbackPostActions, handleErr)
		}
		if handleErr != nil && !errors.Is(handleErr, errRollbackPostActions) {
			handleErr = fmt.Errorf("%w: %w", handleErr, asynq.SkipRetry)
		}
	}()
	var err error

	// Get current deployment (the new deployment record for this rollback)
	j.deployment, err = j.Deps.Repos.Deployment().FindByID(ctx, j.Payload.DeploymentID)
	if err != nil {
		return fmt.Errorf("failed to find current deployment: %w", err)
	}
	completionSaved = j.deployment.Status == sitetypes.DeploymentStatusFinished

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

	if j.deployment.SiteID != j.site.ID {
		return fmt.Errorf("rollback deployment does not belong to site")
	}
	if j.deployment.Status == sitetypes.DeploymentStatusFinished {
		// A previous attempt switched code and saved completion, but may have
		// lost its queue connection before publishing the worker restart event.
		return j.handleRollbackSuccess(ctx, j.deployment, j.site.ID, j.Payload.TargetDeploymentID)
	}
	if j.deployment.Status != sitetypes.DeploymentStatusPending {
		// A worker can disappear without returning SkipRetry. An installing
		// record therefore has an unknown remote outcome and is never safe to
		// switch again automatically, even if a later callback marked it failed.
		return fmt.Errorf("rollback deployment is %s; refusing to repeat the remote switch", j.deployment.Status)
	}

	targetDeployment, err := j.Deps.Repos.Deployment().FindByID(ctx, j.Payload.TargetDeploymentID)
	if err != nil {
		return fmt.Errorf("failed to find target deployment: %w", err)
	}
	if targetDeployment.SiteID != j.site.ID {
		return fmt.Errorf("rollback deployment does not belong to site")
	}
	releaseName, err := resolveDeploymentRelease(ctx, j.Deps.DB, targetDeployment, j.server.ID)
	if err != nil {
		return fmt.Errorf("resolve rollback release: %w", err)
	}

	// A rollback record refers to the same release as its target, so it can
	// itself be selected for a later rollback without inventing a new path.
	claim := j.Deps.DB.WithContext(ctx).Model(&models.Deployment{}).
		Where("id = ? AND site_id = ? AND team_id = ? AND status = ?",
			j.deployment.ID, j.site.ID, j.site.TeamID, sitetypes.DeploymentStatusPending).
		Updates(map[string]any{"release_name": releaseName, "status": sitetypes.DeploymentStatusInstalling})
	if claim.Error != nil {
		return fmt.Errorf("failed to save rollback release: %w", claim.Error)
	}
	if claim.RowsAffected != 1 {
		return errRollbackAlreadyClaimed
	}
	j.deployment.ReleaseName = &releaseName
	j.deployment.Status = sitetypes.DeploymentStatusInstalling

	// Broadcast rollback started
	j.Deps.BroadcastServerEvent(j.server, "deployment.rollback.started", map[string]interface{}{
		"team_id":              j.server.TeamID,
		"site_id":              j.site.ID,
		"deployment_id":        j.deployment.ID,
		"target_deployment_id": targetDeployment.ID,
	})

	releaseDirectory := fmt.Sprintf("%s/releases/%s", j.site.Path, releaseName)
	currentDirectory := fmt.Sprintf("%s/current", j.site.Path)

	// Create rollback task
	task := tasks.RollbackDeployment(tasks.RollbackDeploymentConfig{
		SitePath:         j.site.Path,
		ReleaseDirectory: releaseDirectory,
		CurrentDirectory: currentDirectory,
		Runtime:          tasks.RuntimeActivationConfig{SitePath: j.site.Path, SiteType: j.site.Type, PhpVersion: j.site.PhpVersion},
	})

	j.Deps.Logger.Info().
		Str("site_id", j.site.ID).
		Str("deployment_id", j.deployment.ID).
		Str("target_deployment_id", targetDeployment.ID).
		Str("release_directory", releaseDirectory).
		Msg("Executing rollback")

	// Execute the task on the server as site user (with DB tracking)
	result, err := j.Deps.RunTask(j.server, task).AsUser(j.site.User).ForSite(j.site.ID).TrackInDB().Dispatch(ctx)

	// Preserve the tracked task and its logs even when starting SSH fails.
	if result != nil && result.TaskModel != nil {
		j.deployment.TaskID = &result.TaskModel.ID
		if updateErr := j.Deps.Repos.Deployment().Update(ctx, j.deployment); updateErr != nil {
			j.Deps.Logger.Error().Err(updateErr).Msg("Failed to update deployment with task ID")
		}
	}
	if err == nil && result != nil {
		err = result.Error
		if err == nil && result.TaskResult != nil {
			err = result.TaskResult.Error
			if err == nil && result.TaskResult.TimedOut {
				err = fmt.Errorf("rollback task timed out")
			}
		}
	}
	if err == nil && result == nil {
		err = errors.New("rollback task returned no result")
	}
	if err != nil {
		j.handleRollbackFailure(ctx, j.deployment, j.site.ID, targetDeployment.ID, err)
		return err
	}

	exitCode := result.GetExitCode()

	if exitCode != 0 {
		j.handleRollbackFailure(ctx, j.deployment, j.site.ID, targetDeployment.ID,
			fmt.Errorf("rollback failed with exit code %d", exitCode))
		return fmt.Errorf("rollback failed with exit code %d", exitCode)
	}

	// Success - update deployment status
	return j.handleRollbackSuccess(ctx, j.deployment, j.site.ID, targetDeployment.ID)
}

// handleRollbackSuccess handles successful rollback
func (j *RollbackJob) handleRollbackSuccess(ctx context.Context, deployment *models.Deployment, siteID, targetDeploymentID string) error {
	if deployment.Status != sitetypes.DeploymentStatusFinished {
		deployment.Status = sitetypes.DeploymentStatusFinished
		if err := j.Deps.Repos.Deployment().Update(ctx, deployment); err != nil {
			return fmt.Errorf("failed to save completed rollback: %w", err)
		}
	}

	// Rollback changes the running code just like a deployment. Publish the
	// same durable event so the configured queue/Octane workers restart too.
	taskID := ""
	if deployment.TaskID != nil {
		taskID = *deployment.TaskID
	}
	event, err := siteevents.NewDeploymentSucceeded(siteevents.DeploymentSucceeded{
		DeploymentID: deployment.ID, SiteID: siteID, ServerID: j.server.ID,
		TeamID: j.server.TeamID, TaskID: taskID,
	})
	if err == nil && j.Deps.Queue == nil {
		err = errors.New("queue is unavailable")
	}
	if err == nil {
		err = j.Deps.Dispatch(siteevents.TypeProcessEvent, event,
			asynq.TaskID(pkgjobs.Dedup("site-event", event.Name, event.ID)),
			asynq.MaxRetry(postDeployMaxRetries))
	}
	postActionsPending := err != nil && !errors.Is(err, asynq.ErrTaskIDConflict) && !errors.Is(err, asynq.ErrDuplicateTask)
	// Always publish the durable code-switch result so clients do not remain
	// stuck at installing if post-action publication exhausts its retries.
	data := map[string]any{
		"team_id":              j.server.TeamID,
		"site_id":              siteID,
		"deployment_id":        deployment.ID,
		"target_deployment_id": targetDeploymentID,
		"status":               string(sitetypes.DeploymentStatusFinished),
	}
	if postActionsPending {
		data["post_actions_pending"] = true
		data["warning"] = "Code restored, but worker restart could not be queued."
	}
	j.Deps.BroadcastServerEvent(j.server, "deployment.rollback.completed", data)
	if postActionsPending {
		return fmt.Errorf("%w: %w", errRollbackPostActions, err)
	}

	j.Deps.Logger.Info().
		Str("site_id", siteID).
		Str("deployment_id", deployment.ID).
		Str("target_deployment_id", targetDeploymentID).
		Msg("Rollback completed successfully")
	return nil
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
	if errors.Is(err, errRollbackAlreadyClaimed) {
		// A competing delivery owns the operation and its progress updates.
		return
	}
	if errors.Is(err, errRollbackPostActions) {
		// The code switch succeeded. Keep that result and let the job retry
		// event publication instead of switching the release a second time.
		j.Deps.Logger.Error().Err(err).Str("deployment_id", j.Payload.DeploymentID).Msg("Post-rollback actions could not be queued")
		return
	}
	j.Deps.Logger.Error().Err(err).
		Str("site_id", j.Payload.SiteID).
		Str("deployment_id", j.Payload.DeploymentID).
		Msg("Rollback job failed")

	// Try to update deployment status
	deployment, findErr := j.Deps.Repos.Deployment().FindByID(ctx, j.Payload.DeploymentID)
	if findErr != nil {
		return
	}
	if deployment.Status == sitetypes.DeploymentStatusFinished {
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
	}, asynq.TaskID(pkgjobs.Dedup("rollback", deploymentID)), asynq.MaxRetry(postDeployMaxRetries))
}
