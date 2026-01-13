package repositories

import (
	"context"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/site/models"
	"github.com/kkz6/launch-go/internal/pkg/repository"
)

// RedirectRepository handles database operations for redirects.
// Embeds repository.Base[T] for common CRUD operations.
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
			return nil, ErrRedirectNotFound
		}
		return nil, err
	}
	return redirect, nil
}

// FindBySite finds all redirects for a site
func (r *RedirectRepository) FindBySite(ctx context.Context, siteID string) ([]models.Redirect, error) {
	var redirects []models.Redirect
	err := r.DB.WithContext(ctx).
		Where("site_id = ?", siteID).
		Order("created_at DESC").
		Find(&redirects).Error

	return redirects, err
}

// Note: The following methods are inherited from repository.Base[T]:
// - Create(ctx, entity) error
// - Update(ctx, entity) error
// - Delete(ctx, id) error
// - UpdateFields(ctx, id, fields) error
// - Transaction(ctx, fn) error
