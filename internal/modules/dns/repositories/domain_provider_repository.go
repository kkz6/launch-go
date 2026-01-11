package repositories

import (
	"context"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/dns/models"
)

// DomainProviderRepository handles database operations for domain providers
type DomainProviderRepository struct {
	*BaseRepository
}

// NewDomainProviderRepository creates a new DomainProviderRepository instance
func NewDomainProviderRepository(db *gorm.DB) *DomainProviderRepository {
	return &DomainProviderRepository{
		BaseRepository: NewBaseRepository(db),
	}
}

// Create creates a new domain provider
func (r *DomainProviderRepository) Create(ctx context.Context, provider *models.DomainProvider) error {
	return r.db.WithContext(ctx).Create(provider).Error
}

// FindByID finds a domain provider by ID
func (r *DomainProviderRepository) FindByID(ctx context.Context, id string) (*models.DomainProvider, error) {
	var provider models.DomainProvider
	err := r.db.WithContext(ctx).
		Preload("Domains").
		First(&provider, "id = ?", id).Error
	if err != nil {
		return nil, err
	}

	return &provider, nil
}

// FindByIDAndTeam finds a domain provider by ID and team
func (r *DomainProviderRepository) FindByIDAndTeam(ctx context.Context, id, teamID string) (*models.DomainProvider, error) {
	var provider models.DomainProvider
	err := r.db.WithContext(ctx).
		Preload("Domains").
		First(&provider, "id = ? AND team_id = ?", id, teamID).Error
	if err != nil {
		return nil, err
	}

	return &provider, nil
}

// FindByTeam finds all domain providers for a team
func (r *DomainProviderRepository) FindByTeam(ctx context.Context, teamID string) ([]models.DomainProvider, error) {
	var providers []models.DomainProvider
	err := r.db.WithContext(ctx).
		Where("team_id = ?", teamID).
		Order("created_at DESC").
		Find(&providers).Error

	return providers, err
}

// FindByTeamWithDomainCount finds all domain providers for a team with domain count
func (r *DomainProviderRepository) FindByTeamWithDomainCount(ctx context.Context, teamID string) ([]models.DomainProvider, map[string]int, error) {
	var providers []models.DomainProvider
	err := r.db.WithContext(ctx).
		Preload("Domains").
		Where("team_id = ?", teamID).
		Order("created_at DESC").
		Find(&providers).Error
	if err != nil {
		return nil, nil, err
	}

	counts := make(map[string]int)
	for _, p := range providers {
		counts[p.ID] = len(p.Domains)
	}

	return providers, counts, nil
}

// Update updates a domain provider
func (r *DomainProviderRepository) Update(ctx context.Context, provider *models.DomainProvider) error {
	return r.db.WithContext(ctx).Save(provider).Error
}

// UpdateFields updates specific fields of a domain provider
func (r *DomainProviderRepository) UpdateFields(ctx context.Context, id string, fields map[string]interface{}) error {
	return r.db.WithContext(ctx).
		Model(&models.DomainProvider{}).
		Where("id = ?", id).
		Updates(fields).Error
}

// Delete deletes a domain provider
func (r *DomainProviderRepository) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&models.DomainProvider{}, "id = ?", id).Error
}

// CountDomainsByProvider counts domains for a provider
func (r *DomainProviderRepository) CountDomainsByProvider(ctx context.Context, providerID string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&models.Domain{}).
		Where("domain_provider_id = ?", providerID).
		Count(&count).Error

	return count, err
}
