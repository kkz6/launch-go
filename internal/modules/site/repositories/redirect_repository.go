package repositories

import (
	"context"

	"gorm.io/gorm"

	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"

	"github.com/kkz6/launch-go/internal/modules/site/models"
	"github.com/kkz6/launch-go/internal/pkg/repository"
)

// RedirectRepository handles database operations for redirects.
type RedirectRepository struct {
	repository.Base[models.Redirect]
}

// NewRedirectRepository creates a new redirect repository
func NewRedirectRepository(db *gorm.DB) *RedirectRepository {
	return &RedirectRepository{
		Base: repository.NewBase[models.Redirect](db),
	}
}

// FindByID finds a redirect by ID with custom error.
func (r *RedirectRepository) FindByID(ctx context.Context, id string) (*models.Redirect, error) {
	redirect, err := r.Base.FindByID(ctx, id)
	if err != nil {
		if repository.IsNotFound(err) {
			return nil, fiberutil.NotFound()
		}
		return nil, err
	}
	return redirect, nil
}

// FindBySite finds all redirects for a site
func (r *RedirectRepository) FindBySite(ctx context.Context, siteID string) ([]models.Redirect, error) {
	return repository.FindAll[models.Redirect](ctx, r.DB,
		repository.WithSiteID(siteID),
		repository.OrderByCreatedDesc(),
	)
}

// DeleteBySite deletes all redirects for a site
func (r *RedirectRepository) DeleteBySite(ctx context.Context, siteID string) error {
	return r.DB.WithContext(ctx).
		Where("site_id = ?", siteID).
		Delete(&models.Redirect{}).Error
}

// FindBySiteForCaddy returns active redirects for Caddyfile generation
func (r *RedirectRepository) FindBySiteForCaddy(ctx context.Context, siteID string) ([]models.Redirect, error) {
	return repository.FindAll[models.Redirect](ctx, r.DB,
		repository.WithSiteID(siteID),
		repository.WithStatus("installed"),
		repository.OrderByCreatedAsc(),
	)
}

// FindPendingBySite returns pending redirects for a site
func (r *RedirectRepository) FindPendingBySite(ctx context.Context, siteID string) ([]models.Redirect, error) {
	return repository.FindAll[models.Redirect](ctx, r.DB,
		repository.WithSiteID(siteID),
		repository.WithStatus("pending"),
	)
}

// UpdateStatusBySite updates status of all redirects for a site matching a current status
func (r *RedirectRepository) UpdateStatusBySite(ctx context.Context, siteID, fromStatus, toStatus string) error {
	return r.DB.WithContext(ctx).
		Model(&models.Redirect{}).
		Where("site_id = ? AND status = ?", siteID, fromStatus).
		Update("status", toStatus).Error
}
