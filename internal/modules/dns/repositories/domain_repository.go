package repositories

import (
	"context"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/dns/models"
	"github.com/kkz6/launch-go/internal/pkg/repository"
)

// DomainRepository handles database operations for domains.
// Generic CRUD comes from repository.Base[Domain]; only domain-specific
// queries are defined here.
type DomainRepository struct {
	repository.Base[models.Domain]
}

// NewDomainRepository creates a new DomainRepository instance with the
// Provider and Records relations preloaded by default.
func NewDomainRepository(db *gorm.DB) *DomainRepository {
	return &DomainRepository{
		Base: repository.NewBase[models.Domain](db, "Provider", "Records"),
	}
}

// FindByIDAndTeam finds a domain by ID and team with Records ordered by
// type, name. This shadows Base.FindByIDAndTeam to apply the ordering.
func (r *DomainRepository) FindByIDAndTeam(ctx context.Context, id, teamID string) (*models.Domain, error) {
	return repository.FindOne[models.Domain](ctx, r.DB,
		repository.WithID(id),
		repository.WithTeamID(teamID),
		repository.Preload("Provider"),
		repository.PreloadOrdered("Records", "type ASC, name ASC"),
	)
}

// FindByProvider finds all domains for a provider.
func (r *DomainRepository) FindByProvider(ctx context.Context, providerID string) ([]models.Domain, error) {
	var domains []models.Domain
	err := r.DB.WithContext(ctx).
		Preload("Records").
		Where("domain_provider_id = ?", providerID).
		Find(&domains).Error
	return domains, err
}

// FindByAddressAndProvider finds a domain by address and provider.
func (r *DomainRepository) FindByAddressAndProvider(ctx context.Context, address, providerID string) (*models.Domain, error) {
	var domain models.Domain
	err := r.DB.WithContext(ctx).
		Where("address = ? AND domain_provider_id = ?", address, providerID).
		First(&domain).Error
	if err != nil {
		return nil, err
	}
	return &domain, nil
}

// UpdateOrCreate updates an existing domain matching `where`, or creates one.
func (r *DomainRepository) UpdateOrCreate(ctx context.Context, where map[string]interface{}, update map[string]interface{}) (*models.Domain, error) {
	var domain models.Domain
	err := r.DB.WithContext(ctx).
		Where(where).
		Assign(update).
		FirstOrCreate(&domain).Error
	if err != nil {
		return nil, err
	}

	return &domain, nil
}
