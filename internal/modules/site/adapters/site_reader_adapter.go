package adapters

import (
	"context"
	"errors"

	servercontracts "github.com/kkz6/launch-go/internal/modules/server/contracts"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	"gorm.io/gorm"
)

// SiteReaderAdapter adapts the site repository to the server module's SiteReader interface
type SiteReaderAdapter struct {
	db *gorm.DB
}

// NewSiteReaderAdapter creates a new SiteReaderAdapter
func NewSiteReaderAdapter(db *gorm.DB) servercontracts.SiteReader {
	return &SiteReaderAdapter{db: db}
}

// FindByAddressAndTeam finds all sites matching an address within a team
func (a *SiteReaderAdapter) FindByAddressAndTeam(ctx context.Context, address, teamID string) ([]servercontracts.SiteInfo, error) {
	var sites []models.Site
	err := a.db.WithContext(ctx).
		Where("address = ? AND team_id = ?", address, teamID).
		Find(&sites).Error
	if err != nil {
		return nil, err
	}

	result := make([]servercontracts.SiteInfo, len(sites))
	for i, s := range sites {
		result[i] = toSiteInfo(&s)
	}

	return result, nil
}

// FindByID finds a site by ID and returns a SiteInfo projection
func (a *SiteReaderAdapter) FindByID(ctx context.Context, id string) (*servercontracts.SiteInfo, error) {
	var site models.Site
	err := a.db.WithContext(ctx).First(&site, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	info := toSiteInfo(&site)
	return &info, nil
}

// IsLoadBalanced checks if a site is behind a load balancer
func (a *SiteReaderAdapter) IsLoadBalanced(ctx context.Context, siteID string) (bool, error) {
	var site models.Site
	err := a.db.WithContext(ctx).
		Select("load_balanced_upstream_id").
		First(&site, "id = ?", siteID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}

	return site.IsLoadBalanced(), nil
}

// UpdateLoadBalancedUpstreamID updates the load_balanced_upstream_id for a site
func (a *SiteReaderAdapter) UpdateLoadBalancedUpstreamID(ctx context.Context, siteID string, upstreamID *string) error {
	return a.db.WithContext(ctx).
		Model(&models.Site{}).
		Where("id = ?", siteID).
		Update("load_balanced_upstream_id", upstreamID).Error
}

// toSiteInfo converts a site model to a SiteInfo projection
func toSiteInfo(s *models.Site) servercontracts.SiteInfo {
	return servercontracts.SiteInfo{
		ID:                     s.ID,
		ServerID:               s.ServerID,
		TeamID:                 s.TeamID,
		Address:                s.Address,
		Type:                   string(s.Type),
		LoadBalancedUpstreamID: s.LoadBalancedUpstreamID,
	}
}
