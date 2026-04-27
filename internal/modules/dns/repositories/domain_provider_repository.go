package repositories

import (
	"context"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/dns/models"
	"github.com/kkz6/launch-go/internal/pkg/repository"
)

// DomainProviderRepository handles database operations for domain providers.
// Generic CRUD (Create/FindByID/FindByIDAndTeam/Update/UpdateFields/Delete)
// comes from repository.Base[DomainProvider]; only domain-specific queries
// are defined here.
type DomainProviderRepository struct {
	repository.Base[models.DomainProvider]
}

// NewDomainProviderRepository creates a new DomainProviderRepository
// instance with the Domains relation preloaded by default.
func NewDomainProviderRepository(db *gorm.DB) *DomainProviderRepository {
	return &DomainProviderRepository{
		Base: repository.NewBase[models.DomainProvider](db, "Domains"),
	}
}

// FindByTeamWithDomainCount returns providers for a team along with a
// per-provider count of associated domains.
func (r *DomainProviderRepository) FindByTeamWithDomainCount(ctx context.Context, teamID string) ([]models.DomainProvider, map[string]int, error) {
	var providers []models.DomainProvider
	err := r.DB.WithContext(ctx).
		Preload("Domains").
		Where("team_id = ?", teamID).
		Order("created_at DESC").
		Find(&providers).Error
	if err != nil {
		return nil, nil, err
	}

	counts := make(map[string]int, len(providers))
	for _, p := range providers {
		counts[p.ID] = len(p.Domains)
	}
	return providers, counts, nil
}

// CountDomainsByProvider counts domains for a provider.
func (r *DomainProviderRepository) CountDomainsByProvider(ctx context.Context, providerID string) (int64, error) {
	var count int64
	err := r.DB.WithContext(ctx).
		Model(&models.Domain{}).
		Where("domain_provider_id = ?", providerID).
		Count(&count).Error
	return count, err
}
