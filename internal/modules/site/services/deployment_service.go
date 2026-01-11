package services

import (
	"context"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"

	"github.com/kkz6/launch-go/internal/modules/site/enums"
	"github.com/kkz6/launch-go/internal/modules/site/jobs"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	"github.com/kkz6/launch-go/internal/modules/site/repositories"
	"github.com/kkz6/launch-go/internal/queue"
	"github.com/kkz6/launch-go/internal/websocket"
)

// DeploymentService handles business logic for deployments
type DeploymentService struct {
	*BaseService
}

// NewDeploymentService creates a new deployment service
func NewDeploymentService(
	siteRepo *repositories.SiteRepository,
	deploymentRepo *repositories.DeploymentRepository,
	certificateRepo *repositories.CertificateRepository,
	queueRepo *repositories.QueueRepository,
	commandRepo *repositories.CommandRepository,
	redirectRepo *repositories.RedirectRepository,
	releaseRepo *repositories.ReleaseRepository,
	queueClient *queue.Client,
	ws *websocket.Hub,
	logger *zerolog.Logger,
) *DeploymentService {
	return &DeploymentService{
		BaseService: NewBaseService(
			siteRepo,
			deploymentRepo,
			certificateRepo,
			queueRepo,
			commandRepo,
			redirectRepo,
			releaseRepo,
			queueClient,
			ws,
			logger,
		),
	}
}

// Deploy triggers a new deployment for a site
func (s *DeploymentService) Deploy(ctx context.Context, siteID, serverID, userID string) (*models.Deployment, error) {
	site, err := s.siteRepo.FindByIDAndServer(ctx, siteID, serverID)
	if err != nil {
		return nil, err
	}

	return s.createDeployment(ctx, site, userID, nil)
}

// Rollback rolls back to a previous deployment
func (s *DeploymentService) Rollback(ctx context.Context, siteID, serverID, targetDeploymentID, userID string) (*models.Deployment, error) {
	site, err := s.siteRepo.FindByIDAndServer(ctx, siteID, serverID)
	if err != nil {
		return nil, err
	}

	if !site.ZeroDowntimeDeployment {
		return nil, ErrRollbackNotSupported
	}

	targetDeployment, err := s.deploymentRepo.FindByID(ctx, targetDeploymentID)
	if err != nil {
		return nil, err
	}

	if targetDeployment.SiteID != site.ID {
		return nil, ErrDeploymentNotBelongToSite
	}

	if targetDeployment.Status != enums.DeploymentStatusFinished {
		return nil, ErrInvalidRollbackTarget
	}

	// Check for active deployment
	activeDeployment, _ := s.deploymentRepo.FindActiveBySite(ctx, site.ID)
	if activeDeployment != nil {
		return nil, ErrPendingDeployment
	}

	// Get latest deployment for rollback metadata
	latestDeployment, _ := s.deploymentRepo.FindLatestBySite(ctx, site.ID)

	// Create rollback deployment
	commitData := map[string]interface{}{
		"rollback_from": latestDeployment.ID,
		"rollback_to":   targetDeployment.ID,
	}

	if targetDeployment.CommitData != nil && *targetDeployment.CommitData != "" {
		existingData := targetDeployment.GetCommitData()
		for k, v := range existingData {
			if k != "rollback_from" && k != "rollback_to" {
				commitData[k] = v
			}
		}
	}

	deployment := &models.Deployment{
		SiteID:  site.ID,
		UserID:  &userID,
		Status:  enums.DeploymentStatusPending,
		GitHash: targetDeployment.GitHash,
	}

	if err := deployment.SetCommitData(commitData); err != nil {
		return nil, err
	}

	if err := s.deploymentRepo.Create(ctx, deployment); err != nil {
		return nil, err
	}

	// Dispatch rollback job
	task, err := jobs.NewRollbackTask(site.ID, deployment.ID, targetDeploymentID, userID)
	if err != nil {
		return nil, err
	}

	if s.queue != nil {
		if _, err := s.queue.EnqueueDefault(task); err != nil {
			s.logger.Error().Err(err).Str("deployment_id", deployment.ID).Msg("Failed to enqueue rollback job")
		}
	}

	s.logger.Info().
		Str("site_id", site.ID).
		Str("deployment_id", deployment.ID).
		Str("target_deployment_id", targetDeploymentID).
		Msg("Rollback initiated")

	return deployment, nil
}

// createDeployment creates a new deployment for a site
func (s *DeploymentService) createDeployment(ctx context.Context, site *models.Site, userID string, commitData map[string]interface{}) (*models.Deployment, error) {
	// Check for active deployment
	activeDeployment, _ := s.deploymentRepo.FindActiveBySite(ctx, site.ID)
	if activeDeployment != nil {
		if site.QueueDeployments {
			// Queue the deployment
			deployment := &models.Deployment{
				SiteID: site.ID,
				UserID: &userID,
				Status: enums.DeploymentStatusQueued,
			}

			if commitData != nil {
				if err := deployment.SetCommitData(commitData); err != nil {
					return nil, err
				}
			}

			if err := s.deploymentRepo.Create(ctx, deployment); err != nil {
				return nil, err
			}

			s.logger.Info().
				Str("site_id", site.ID).
				Str("deployment_id", deployment.ID).
				Msg("Deployment queued")

			return deployment, nil
		}

		return nil, ErrPendingDeployment
	}

	deployment := &models.Deployment{
		SiteID: site.ID,
		UserID: &userID,
		Status: enums.DeploymentStatusPending,
	}

	if commitData != nil {
		if err := deployment.SetCommitData(commitData); err != nil {
			return nil, err
		}
	}

	if err := s.deploymentRepo.Create(ctx, deployment); err != nil {
		return nil, err
	}

	// Dispatch deployment job
	var task *asynq.Task
	var dispatchErr error

	if site.ZeroDowntimeDeployment {
		task, dispatchErr = jobs.NewDeployZeroDowntimeTask(site.ID, deployment.ID, userID)
	} else {
		task, dispatchErr = jobs.NewDeployTask(site.ID, deployment.ID, userID)
	}

	if dispatchErr != nil {
		return nil, dispatchErr
	}

	if s.queue != nil {
		if _, err := s.queue.EnqueueDefault(task); err != nil {
			s.logger.Error().Err(err).Str("deployment_id", deployment.ID).Msg("Failed to enqueue deployment job")
		}
	}

	s.logger.Info().
		Str("site_id", site.ID).
		Str("deployment_id", deployment.ID).
		Msg("Deployment started")

	return deployment, nil
}

// List returns all deployments for a site
func (s *DeploymentService) List(ctx context.Context, siteID, serverID string) ([]models.Deployment, error) {
	// Verify site exists and belongs to server
	if _, err := s.siteRepo.FindByIDAndServer(ctx, siteID, serverID); err != nil {
		return nil, err
	}

	return s.deploymentRepo.FindBySite(ctx, siteID)
}

// FindByID finds a deployment by ID
func (s *DeploymentService) FindByID(ctx context.Context, id, siteID, serverID string) (*models.Deployment, error) {
	// Verify site exists and belongs to server
	if _, err := s.siteRepo.FindByIDAndServer(ctx, siteID, serverID); err != nil {
		return nil, err
	}

	return s.deploymentRepo.FindByIDAndSite(ctx, id, siteID)
}

// ProcessNextQueued processes the next queued deployment
func (s *DeploymentService) ProcessNextQueued(ctx context.Context, siteID string) (*models.Deployment, error) {
	site, err := s.siteRepo.FindByID(ctx, siteID)
	if err != nil {
		return nil, err
	}

	// Check for active deployment
	activeDeployment, _ := s.deploymentRepo.FindActiveBySite(ctx, site.ID)
	if activeDeployment != nil {
		return nil, nil
	}

	// Get next queued deployment
	queuedDeployments, err := s.deploymentRepo.FindQueuedBySite(ctx, site.ID)
	if err != nil {
		return nil, err
	}

	if len(queuedDeployments) == 0 {
		return nil, nil
	}

	deployment := &queuedDeployments[0]
	deployment.Status = enums.DeploymentStatusPending

	if err := s.deploymentRepo.Update(ctx, deployment); err != nil {
		return nil, err
	}

	// Dispatch deployment job
	var task *asynq.Task
	var dispatchErr error

	if site.ZeroDowntimeDeployment {
		task, dispatchErr = jobs.NewDeployZeroDowntimeTask(site.ID, deployment.ID, "")
	} else {
		task, dispatchErr = jobs.NewDeployTask(site.ID, deployment.ID, "")
	}

	if dispatchErr != nil {
		return nil, dispatchErr
	}

	if s.queue != nil {
		if _, err := s.queue.EnqueueDefault(task); err != nil {
			s.logger.Error().Err(err).Str("deployment_id", deployment.ID).Msg("Failed to enqueue deployment job")
		}
	}

	return deployment, nil
}

// GetQueuedCount returns the count of queued deployments
func (s *DeploymentService) GetQueuedCount(ctx context.Context, siteID string) (int64, error) {
	return s.deploymentRepo.CountQueuedBySite(ctx, siteID)
}

// CancelQueued cancels all queued deployments
func (s *DeploymentService) CancelQueued(ctx context.Context, siteID, serverID string) (int64, error) {
	// Verify site exists and belongs to server
	if _, err := s.siteRepo.FindByIDAndServer(ctx, siteID, serverID); err != nil {
		return 0, err
	}

	return s.deploymentRepo.CancelQueued(ctx, siteID)
}

// EnableAutoDeployment enables auto-deployment for a site
func (s *DeploymentService) EnableAutoDeployment(ctx context.Context, siteID, serverID string) error {
	site, err := s.siteRepo.FindByIDAndServer(ctx, siteID, serverID)
	if err != nil {
		return err
	}

	if site.SourceControlID == nil || site.SourceControlRepositoriesID == nil {
		return ErrSourceControlNotConnected
	}

	site.AutoDeployment = true

	return s.siteRepo.Update(ctx, site)
}

// DisableAutoDeployment disables auto-deployment for a site
func (s *DeploymentService) DisableAutoDeployment(ctx context.Context, siteID, serverID string) error {
	return s.siteRepo.UpdateFields(ctx, siteID, map[string]interface{}{
		"auto_deployment": false,
	})
}

// BroadcastProgress broadcasts deployment progress
func (s *DeploymentService) BroadcastProgress(siteID, deploymentID, status, message string) {
	s.ws.BroadcastToDeployment(deploymentID, "deployment.progress", map[string]interface{}{
		"site_id":       siteID,
		"deployment_id": deploymentID,
		"status":        status,
		"message":       message,
	})
}
