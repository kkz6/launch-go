package repositories

import (
	"context"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/dns/models"
	"github.com/kkz6/launch-go/internal/pkg/repository"
)

// DomainRepository handles database operations for domains
type DomainRepository struct {
	repository.Base[models.Domain]
}

// NewDomainRepository creates a new DomainRepository instance
func NewDomainRepository(db *gorm.DB) *DomainRepository {
	return &DomainRepository{
		Base: repository.NewBase[models.Domain](db),
	}
}

// Create creates a new domain
func (r *DomainRepository) Create(ctx context.Context, domain *models.Domain) error {
	return r.DB.WithContext(ctx).Create(domain).Error
}

// FindByID finds a domain by ID
func (r *DomainRepository) FindByID(ctx context.Context, id string) (*models.Domain, error) {
	var domain models.Domain
	err := r.DB.WithContext(ctx).
		Preload("Provider").
		Preload("Records").
		First(&domain, "id = ?", id).Error
	if err != nil {
		return nil, err
	}

	return &domain, nil
}

// FindByIDAndTeam finds a domain by ID and team
func (r *DomainRepository) FindByIDAndTeam(ctx context.Context, id, teamID string) (*models.Domain, error) {
	var domain models.Domain
	err := r.DB.WithContext(ctx).
		Preload("Provider").
		Preload("Records", func(db *gorm.DB) *gorm.DB {
			return db.Order("type ASC, name ASC")
		}).
		First(&domain, "id = ? AND team_id = ?", id, teamID).Error
	if err != nil {
		return nil, err
	}

	return &domain, nil
}

// FindByTeam finds all domains for a team
func (r *DomainRepository) FindByTeam(ctx context.Context, teamID string) ([]models.Domain, error) {
	var domains []models.Domain
	err := r.DB.WithContext(ctx).
		Preload("Provider").
		Preload("Records").
		Where("team_id = ?", teamID).
		Order("created_at DESC").
		Find(&domains).Error

	return domains, err
}

// FindByProvider finds all domains for a provider
func (r *DomainRepository) FindByProvider(ctx context.Context, providerID string) ([]models.Domain, error) {
	var domains []models.Domain
	err := r.DB.WithContext(ctx).
		Preload("Records").
		Where("domain_provider_id = ?", providerID).
		Find(&domains).Error

	return domains, err
}

// FindByAddressAndProvider finds a domain by address and provider
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

// Update updates a domain
func (r *DomainRepository) Update(ctx context.Context, domain *models.Domain) error {
	return r.DB.WithContext(ctx).Save(domain).Error
}

// UpdateOrCreate updates or creates a domain
func (r *DomainRepository) UpdateOrCreate(ctx context.Context, where map[string]interface{}, update map[string]interface{}) (*models.Domain, error) {
	var domain models.Domain

	// First try to find existing
	err := r.DB.WithContext(ctx).Where(where).First(&domain).Error
	if err == gorm.ErrRecordNotFound {
		// Create new domain with all values
		for k, v := range where {
			update[k] = v
		}
		domain = models.Domain{}
		if err := r.DB.WithContext(ctx).Model(&domain).Create(update).Error; err != nil {
			return nil, err
		}
		// Reload the domain
		return r.FindByAddressAndProvider(ctx, where["address"].(string), where["domain_provider_id"].(string))
	}

	if err != nil {
		return nil, err
	}

	// Update existing
	if err := r.DB.WithContext(ctx).Model(&domain).Updates(update).Error; err != nil {
		return nil, err
	}

	return &domain, nil
}

// Delete deletes a domain
func (r *DomainRepository) Delete(ctx context.Context, id string) error {
	return r.DB.WithContext(ctx).Delete(&models.Domain{}, "id = ?", id).Error
}
