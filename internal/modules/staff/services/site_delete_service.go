package services

import (
	"context"
	"errors"

	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
)

var ErrSiteNotFound = errors.New("site not found")

type SiteDeleter interface {
	DeleteSite(ctx context.Context, id, serverID, teamID, actorID string) error
	RemoveSiteRecord(ctx context.Context, id, serverID, teamID, actorID string) error
}

func (s *Service) SetSiteDeleter(deleter SiteDeleter) { s.siteDeleter = deleter }

type SiteDeletionResult struct {
	SiteID string `json:"site_id"`
	Status string `json:"status"`
}

// DeleteSite uses stored ownership, never the staff member's current team.
func (s *Service) DeleteSite(ctx context.Context, id, actorID, confirmation string, removeRecord bool) (SiteDeletionResult, error) {
	var result SiteDeletionResult
	site, err := s.repos.SiteForDeletion(ctx, id)
	if err != nil {
		return result, err
	}
	if site == nil {
		return result, ErrSiteNotFound
	}
	if confirmation == "" || confirmation != site.Address {
		return result, fiberutil.Validation("Type the site domain exactly to confirm deletion")
	}
	if actorID == "" {
		return result, fiberutil.Unauthorized()
	}
	if s.siteDeleter == nil {
		return result, fiberutil.Internal("Site deletion is not configured")
	}
	if removeRecord {
		if err := s.siteDeleter.RemoveSiteRecord(ctx, site.ID, site.ServerID, site.TeamID, actorID); err != nil {
			return result, err
		}
		return SiteDeletionResult{SiteID: site.ID, Status: "removed"}, nil
	}
	if err := s.siteDeleter.DeleteSite(ctx, site.ID, site.ServerID, site.TeamID, actorID); err != nil {
		return result, err
	}
	return SiteDeletionResult{SiteID: site.ID, Status: "deletion_requested"}, nil
}
