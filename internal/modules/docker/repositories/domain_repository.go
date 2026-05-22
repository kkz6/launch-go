package repositories

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/docker/models"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/repository"
)

// DomainRepository handles persistence for application domains.
type DomainRepository struct {
	repository.Base[models.ApplicationDomain]
}

// NewDomainRepository wires the base repository for application domains.
func NewDomainRepository(db *gorm.DB) *DomainRepository {
	return &DomainRepository{Base: repository.NewBase[models.ApplicationDomain](db)}
}

// FindByID returns a domain by ID, mapping NotFound to a HTTP 404.
func (r *DomainRepository) FindByID(ctx context.Context, id string) (*models.ApplicationDomain, error) {
	var d models.ApplicationDomain
	err := r.DB.WithContext(ctx).Where("id = ?", id).First(&d).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fiberutil.NotFound()
		}
		return nil, err
	}
	return &d, nil
}

// ListForApplication returns every live domain attached to an application,
// ordered by host so the UI list is stable across reloads.
func (r *DomainRepository) ListForApplication(
	ctx context.Context, applicationID string,
) ([]models.ApplicationDomain, error) {
	var rows []models.ApplicationDomain
	err := r.DB.WithContext(ctx).
		Where("application_id = ?", applicationID).
		Order("host ASC").
		Find(&rows).Error
	return rows, err
}

// ExistsByHost returns true if a live domain with this host is already
// attached to the application. The migration also has a unique index on
// (application_id, host, deleted_at) — this check just lets the service
// return a friendlier 409 than a generic 500.
func (r *DomainRepository) ExistsByHost(
	ctx context.Context, applicationID, host string,
) (bool, error) {
	var count int64
	err := r.DB.WithContext(ctx).
		Model(&models.ApplicationDomain{}).
		Where("application_id = ? AND host = ?", applicationID, host).
		Count(&count).Error
	return count > 0, err
}
