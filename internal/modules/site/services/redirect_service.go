package services

import (
	"context"

	"github.com/rs/zerolog"

	"github.com/kkz6/launch-go/internal/modules/site/dto"
	"github.com/kkz6/launch-go/internal/modules/site/enums"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	"github.com/kkz6/launch-go/internal/modules/site/repositories"
	"github.com/kkz6/launch-go/internal/queue"
	"github.com/kkz6/launch-go/internal/websocket"
)

// RedirectService handles business logic for redirects
type RedirectService struct {
	*BaseService
}

// NewRedirectService creates a new redirect service
func NewRedirectService(
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
) *RedirectService {
	return &RedirectService{
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

// Create creates a new redirect
func (s *RedirectService) Create(ctx context.Context, siteID, serverID, userID string, req *dto.CreateRedirectRequest) (*models.Redirect, error) {
	if _, err := s.siteRepo.FindByIDAndServer(ctx, siteID, serverID); err != nil {
		return nil, err
	}

	redirect := &models.Redirect{
		SiteID: siteID,
		UserID: userID,
		Mode:   enums.RedirectMode(req.Mode),
		From:   req.From,
		To:     req.To,
		Status: "pending",
	}

	if err := s.redirectRepo.Create(ctx, redirect); err != nil {
		return nil, err
	}

	// TODO: Dispatch Caddyfile update job

	return redirect, nil
}

// List returns all redirects for a site
func (s *RedirectService) List(ctx context.Context, siteID, serverID string) ([]models.Redirect, error) {
	if _, err := s.siteRepo.FindByIDAndServer(ctx, siteID, serverID); err != nil {
		return nil, err
	}

	return s.redirectRepo.FindBySite(ctx, siteID)
}

// Delete deletes a redirect
func (s *RedirectService) Delete(ctx context.Context, redirectID, siteID, serverID string) error {
	if _, err := s.siteRepo.FindByIDAndServer(ctx, siteID, serverID); err != nil {
		return err
	}

	if _, err := s.redirectRepo.FindByID(ctx, redirectID); err != nil {
		return err
	}

	// TODO: Dispatch Caddyfile update job

	return s.redirectRepo.Delete(ctx, redirectID)
}
