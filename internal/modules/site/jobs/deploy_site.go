package jobs

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/site/enums"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	"github.com/kkz6/launch-go/internal/websocket"
)

const TypeDeploySite = "site:deploy"

// DeploySitePayload contains data for deploying a site
type DeploySitePayload struct {
	DeploymentID         string            `json:"deployment_id"`
	EnvironmentVariables map[string]string `json:"environment_variables,omitempty"`
}

// DeploySiteJob handles site deployment
type DeploySiteJob struct {
	db     *gorm.DB
	ws     *websocket.Hub
	logger *zerolog.Logger
}

// NewDeploySiteJob creates a new deploy site job handler
func NewDeploySiteJob(db *gorm.DB, ws *websocket.Hub, logger *zerolog.Logger) *DeploySiteJob {
	return &DeploySiteJob{
		db:     db,
		ws:     ws,
		logger: logger,
	}
}

// NewDeploySiteTask creates a new asynq task for deploying a site
func NewDeploySiteTask(deploymentID string, environmentVariables map[string]string) (*asynq.Task, error) {
	payload, err := json.Marshal(DeploySitePayload{
		DeploymentID:         deploymentID,
		EnvironmentVariables: environmentVariables,
	})
	if err != nil {
		return nil, err
	}

	return asynq.NewTask(TypeDeploySite, payload), nil
}

// Handle processes the deploy site job
func (j *DeploySiteJob) Handle(ctx context.Context, t *asynq.Task) error {
	var payload DeploySitePayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	j.logger.Info().
		Str("deployment_id", payload.DeploymentID).
		Msg("Starting site deployment")

	// Fetch the deployment with site
	var deployment models.Deployment
	if err := j.db.Preload("Site").First(&deployment, "id = ?", payload.DeploymentID).Error; err != nil {
		return fmt.Errorf("failed to find deployment: %w", err)
	}

	if deployment.Site == nil {
		return fmt.Errorf("deployment has no associated site")
	}

	// Update deployment status to installing
	if err := j.db.Model(&deployment).Update("status", enums.DeploymentStatusInstalling).Error; err != nil {
		return fmt.Errorf("failed to update deployment status: %w", err)
	}

	// Determine task type based on zero downtime setting
	taskType := "deploy"
	if deployment.Site.ZeroDowntimeDeployment {
		taskType = "deploy_zero_downtime"
	}

	j.broadcastProgress(deployment.Site.ID, deployment.ID, "installing", "Starting deployment...")

	// TODO: Implement actual deployment logic based on taskType:
	// 1. Create new release directory (for zero downtime)
	// 2. Clone/checkout repository
	// 3. Copy shared files (.env, storage)
	// 4. Install Composer dependencies
	// 5. Install NPM dependencies
	// 6. Build assets
	// 7. Run Laravel optimizations (config:cache, route:cache, etc.)
	// 8. Run migrations
	// 9. Update current symlink (for zero downtime)
	// 10. Restart PHP-FPM
	// 11. Restart queue workers
	// 12. Clean up old releases

	j.logger.Info().
		Str("deployment_id", payload.DeploymentID).
		Str("task_type", taskType).
		Msg("Deployment task type determined")

	j.broadcastProgress(deployment.Site.ID, deployment.ID, "finished", "Deployment completed successfully")

	// Update deployment status to finished
	if err := j.db.Model(&deployment).Update("status", enums.DeploymentStatusFinished).Error; err != nil {
		return fmt.Errorf("failed to update deployment status: %w", err)
	}

	return nil
}

// Failed handles job failure
func (j *DeploySiteJob) Failed(ctx context.Context, t *asynq.Task, err error) {
	var payload DeploySitePayload
	if unmarshalErr := json.Unmarshal(t.Payload(), &payload); unmarshalErr != nil {
		j.logger.Error().Err(unmarshalErr).Msg("Failed to unmarshal payload in failure handler")
		return
	}

	j.logger.Error().
		Err(err).
		Str("deployment_id", payload.DeploymentID).
		Msg("Site deployment failed")

	// Fetch deployment for broadcasting
	var deployment models.Deployment
	if dbErr := j.db.Preload("Site").First(&deployment, "id = ?", payload.DeploymentID).Error; dbErr != nil {
		j.logger.Error().Err(dbErr).Msg("Failed to find deployment in failure handler")
		return
	}

	// Update deployment status to failed
	j.db.Model(&models.Deployment{}).
		Where("id = ?", payload.DeploymentID).
		Update("status", enums.DeploymentStatusFailed)

	if deployment.Site != nil {
		j.broadcastProgress(deployment.Site.ID, deployment.ID, "failed", "Deployment failed")
	}
}

func (j *DeploySiteJob) broadcastProgress(siteID, deploymentID, status, message string) {
	j.ws.BroadcastToDeployment(deploymentID, "deployment.progress", map[string]interface{}{
		"site_id":       siteID,
		"deployment_id": deploymentID,
		"status":        status,
		"message":       message,
	})
}
