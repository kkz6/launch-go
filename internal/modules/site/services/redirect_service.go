package services

import (
	"context"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/site/dto"
	"github.com/kkz6/launch-go/internal/modules/site/jobs"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	"github.com/kkz6/launch-go/internal/pkg/launch/activity"
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
		Mode:   req.Mode,
		From:   req.From,
		To:     req.To,
		Status: "pending",
	}
	redirect.SiteID = site.ID
	redirect.TeamID = site.TeamID
	redirect.UserID = userID

	if err := s.Repos().Redirect().Create(ctx, redirect); err != nil {
		return nil, err
	}

	activity.RecordWithLog(ctx, "site", "created", "", redirect, "Redirect was created")

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

	activity.RecordWithLog(ctx, "site", "deleted", "", redirect, "Redirect was deleted")

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

	s.DispatchTask("UpdateCaddyfile", func() (*asynq.Task, error) {
		return jobs.NewUpdateCaddyfileTask(siteID, userIDPtr)
	}, "site_id", siteID)
}
