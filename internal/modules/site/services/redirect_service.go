package services

import (
	"context"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/site/dto"
	"github.com/kkz6/launch-go/internal/modules/site/jobs"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/launch/activity"
)

// RedirectService handles business logic for redirects.
type RedirectService struct {
	*BaseService
}

// NewRedirectService creates a new redirect service.
func NewRedirectService(deps *ServiceDeps) *RedirectService {
	return &RedirectService{BaseService: NewBaseService(deps)}
}

// Create creates a new redirect under a site. Signature matches
// CreateDoubleNestedFunc: (ctx, parentID=siteID, grandparentID=serverID,
// teamID, userID, req).
func (s *RedirectService) Create(ctx context.Context, siteID, serverID, teamID, userID string, req *dto.CreateRedirectRequest) (dto.RedirectResponse, error) {
	_ = teamID
	site, err := s.Repos().Site().FindByIDAndServer(ctx, siteID, serverID)
	if err != nil {
		return dto.RedirectResponse{}, err
	}

	redirect := &models.Redirect{
		Mode:   req.Type,
		From:   req.From,
		To:     req.To,
		Status: "pending",
	}
	redirect.SiteID = site.ID
	redirect.TeamID = site.TeamID
	redirect.UserID = userID

	if err := s.Repos().Redirect().Create(ctx, redirect); err != nil {
		return dto.RedirectResponse{}, err
	}

	activity.RecordWithLog(ctx, "site", "created", userID, redirect, "Redirect was created")
	s.dispatchCaddyfileUpdate(site.ID, userID)
	return dto.ToRedirectResponse(redirect), nil
}

// List returns all redirects for a site. Signature matches IndexDoubleNestedFunc.
func (s *RedirectService) List(ctx context.Context, siteID, serverID, teamID string) ([]dto.RedirectResponse, error) {
	_ = teamID
	if _, err := s.Repos().Site().FindByIDAndServer(ctx, siteID, serverID); err != nil {
		return nil, err
	}
	redirects, err := s.Repos().Redirect().FindBySite(ctx, siteID)
	if err != nil {
		return nil, err
	}
	out := make([]dto.RedirectResponse, len(redirects))
	for i := range redirects {
		out[i] = dto.ToRedirectResponse(&redirects[i])
	}
	return out, nil
}

// Update edits an existing redirect (from/to/type) and re-triggers
// the Caddyfile update so the on-server reverse proxy picks up the
// new rule. Signature matches UpdateDoubleNestedFunc:
// (ctx, redirectID, parent=siteID, grandparent=serverID, teamID, userID, req).
//
// Status is reset to "pending" so the Caddyfile update job can flip
// it back to "installed" on success — mirrors the Create lifecycle.
func (s *RedirectService) Update(ctx context.Context, redirectID, siteID, serverID, teamID, userID string, req *dto.UpdateRedirectRequest) (dto.RedirectResponse, error) {
	_ = teamID
	if _, err := s.Repos().Site().FindByIDAndServer(ctx, siteID, serverID); err != nil {
		return dto.RedirectResponse{}, err
	}
	redirect, err := s.Repos().Redirect().FindByID(ctx, redirectID)
	if err != nil {
		return dto.RedirectResponse{}, err
	}
	if redirect.SiteID != siteID {
		return dto.RedirectResponse{}, fiberutil.NotFound()
	}

	if req.From != nil {
		redirect.From = *req.From
	}
	if req.To != nil {
		redirect.To = *req.To
	}
	if req.Type != nil {
		redirect.Mode = *req.Type
	}
	// Flip back to pending so the caddyfile-update job re-applies it.
	redirect.Status = "pending"

	if err := s.Repos().Redirect().Update(ctx, redirect); err != nil {
		return dto.RedirectResponse{}, err
	}

	activity.RecordWithLog(ctx, "site", "updated", userID, redirect, "Redirect was updated")
	s.dispatchCaddyfileUpdate(redirect.SiteID, userID)
	return dto.ToRedirectResponse(redirect), nil
}

// Delete deletes a redirect. Signature matches DeleteDoubleNestedFunc.
func (s *RedirectService) Delete(ctx context.Context, redirectID, siteID, serverID, teamID, userID string) error {
	_ = teamID
	if _, err := s.Repos().Site().FindByIDAndServer(ctx, siteID, serverID); err != nil {
		return err
	}

	redirect, err := s.Repos().Redirect().FindByID(ctx, redirectID)
	if err != nil {
		return err
	}

	activity.RecordWithLog(ctx, "site", "deleted", userID, redirect, "Redirect was deleted")

	if err := s.Repos().Redirect().Delete(ctx, redirectID); err != nil {
		return err
	}

	s.dispatchCaddyfileUpdate(redirect.SiteID, userID)
	return nil
}

// dispatchCaddyfileUpdate dispatches a Caddyfile update job for the site.
func (s *RedirectService) dispatchCaddyfileUpdate(siteID, userID string) {
	s.DispatchTask("UpdateCaddyfile", func() (*asynq.Task, error) {
		return jobs.NewUpdateCaddyfileTask(siteID, stringToPtr(userID))
	}, "site_id", siteID)
}
