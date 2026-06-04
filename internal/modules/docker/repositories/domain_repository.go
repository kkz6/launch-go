package repositories

import (
	"context"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/docker/models"
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
	return repository.FindOne[models.ApplicationDomain](ctx, r.DB,
		repository.WithID(id),
	)
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

// ListForCompose mirrors ListForApplication on the compose owner
// column. Ordered by host for a stable UI list. We also sort by
// service_name as a secondary key so two domains on the same host
// but different services render in a deterministic order.
func (r *DomainRepository) ListForCompose(
	ctx context.Context, composeID string,
) ([]models.ApplicationDomain, error) {
	var rows []models.ApplicationDomain
	err := r.DB.WithContext(ctx).
		Where("compose_id = ?", composeID).
		Order("host ASC, service_name ASC").
		Find(&rows).Error
	return rows, err
}

// ExistsByHostForCompose mirrors ExistsByHost but scoped to a
// compose stack. Uniqueness is per-owner, so an application and a
// compose stack can both attach the same host without collision —
// which is fine in theory (different Traefik file targets) but
// would be confusing in practice; we leave that combination to the
// operator to spot.
func (r *DomainRepository) ExistsByHostForCompose(
	ctx context.Context, composeID, host string,
) (bool, error) {
	var count int64
	err := r.DB.WithContext(ctx).
		Model(&models.ApplicationDomain{}).
		Where("compose_id = ? AND host = ?", composeID, host).
		Count(&count).Error
	return count > 0, err
}
