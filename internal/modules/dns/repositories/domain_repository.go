package repositories

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/dns/models"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
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
	var domain models.Domain
	err := r.DB.WithContext(ctx).
		Preload("Provider").
		Preload("Records", func(db *gorm.DB) *gorm.DB {
			return db.Order("type ASC, name ASC")
		}).
		First(&domain, "id = ? AND team_id = ?", id, teamID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fiberutil.NotFound()
		}
		return nil, err
	}
	return &domain, nil
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

	err := r.DB.WithContext(ctx).Where(where).First(&domain).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		for k, v := range where {
			update[k] = v
		}
		domain = models.Domain{}
		if err := r.DB.WithContext(ctx).Model(&domain).Create(update).Error; err != nil {
			return nil, err
		}
		return r.FindByAddressAndProvider(ctx, where["address"].(string), where["domain_provider_id"].(string))
	}
	if err != nil {
		return nil, err
	}

	if err := r.DB.WithContext(ctx).Model(&domain).Updates(update).Error; err != nil {
		return nil, err
	}
	return &domain, nil
}
