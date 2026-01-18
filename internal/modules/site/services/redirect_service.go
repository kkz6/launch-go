package services

import (
	"context"

	"github.com/kkz6/launch-go/internal/modules/site/dto"
	"github.com/kkz6/launch-go/internal/modules/site/jobs"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	"github.com/kkz6/launch-go/internal/pkg/activity"
)

// RedirectService handles business logic for redirects
type RedirectService struct {
	*BaseService
}

// NewRedirectService creates a new redirect service
func NewRedirectService(deps *ServiceDeps) *RedirectService {
	return &RedirectService{
		BaseService: NewBaseService(deps),
	}
}

// Create creates a new redirect
func (s *RedirectService) Create(ctx context.Context, siteID, serverID, userID string, req *dto.CreateRedirectRequest) (*models.Redirect, error) {
	site, err := s.Repos().Site().FindByIDAndServer(ctx, siteID, serverID)
	if err != nil {
		return nil, err
	}

	redirect := &models.Redirect{
		SiteID: site.ID,
		TeamID: site.TeamID,
		UserID: userID,
		Mode:   req.Mode,
		From:   req.From,
		To:     req.To,
		Status: "pending",
	}

	if err := s.Repos().Redirect().Create(ctx, redirect); err != nil {
		return nil, err
	}

	activity.New(s.Repos().Redirect().DB).
		WithContext(ctx).
		UseLog("site").
		On(redirect).
		WithEvent("created").
		Log("Redirect was created")

	// Dispatch Caddyfile update job
	s.dispatchCaddyfileUpdate(site.ID, userID)

	return redirect, nil
}

// List returns all redirects for a site
func (s *RedirectService) List(ctx context.Context, siteID, serverID string) ([]models.Redirect, error) {
	if _, err := s.Repos().Site().FindByIDAndServer(ctx, siteID, serverID); err != nil {
		return nil, err
	}

	return s.Repos().Redirect().FindBySite(ctx, siteID)
}

// Delete deletes a redirect
func (s *RedirectService) Delete(ctx context.Context, redirectID, siteID, serverID string) error {
	if _, err := s.Repos().Site().FindByIDAndServer(ctx, siteID, serverID); err != nil {
		return err
	}

	redirect, err := s.Repos().Redirect().FindByID(ctx, redirectID)
	if err != nil {
		return err
	}

	activity.New(s.Repos().Redirect().DB).
		WithContext(ctx).
		UseLog("site").
		On(redirect).
		WithEvent("deleted").
		Log("Redirect was deleted")

	if err := s.Repos().Redirect().Delete(ctx, redirectID); err != nil {
		return err
	}

	// Dispatch Caddyfile update job after deletion
	s.dispatchCaddyfileUpdate(redirect.SiteID, "")

	return nil
}

// dispatchCaddyfileUpdate dispatches a Caddyfile update job for the site
func (s *RedirectService) dispatchCaddyfileUpdate(siteID, userID string) {
	var userIDPtr *string
	if userID != "" {
		userIDPtr = &userID
	}

	task, err := jobs.NewUpdateCaddyfileTask(siteID, userIDPtr)
	if err != nil {
		s.LogError(err, "Failed to create update Caddyfile task", "site_id", siteID)
		return
	}

	if s.Queue != nil {
		if _, err := s.Queue.Enqueue(task); err != nil {
			s.LogError(err, "Failed to enqueue update Caddyfile job", "site_id", siteID)
		}
	}
}
