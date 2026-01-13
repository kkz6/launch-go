package services

import (
	"context"

	"github.com/rs/zerolog"

	"github.com/kkz6/launch-go/internal/modules/site/dto"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	"github.com/kkz6/launch-go/internal/modules/site/repositories"
	"github.com/kkz6/launch-go/internal/pkg/activity"
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
		Mode:   req.Mode,
		From:   req.From,
		To:     req.To,
		Status: "pending",
	}

	if err := s.redirectRepo.Create(ctx, redirect); err != nil {
		return nil, err
	}

	activity.New(s.redirectRepo.DB()).
		WithContext(ctx).
		UseLog("site").
		On(redirect).
		WithEvent("created").
		Log("Redirect was created")

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

	redirect, err := s.redirectRepo.FindByID(ctx, redirectID)
	if err != nil {
		return err
	}

	activity.New(s.redirectRepo.DB()).
		WithContext(ctx).
		UseLog("site").
		On(redirect).
		WithEvent("deleted").
		Log("Redirect was deleted")

	// TODO: Dispatch Caddyfile update job

	return s.redirectRepo.Delete(ctx, redirectID)
}
