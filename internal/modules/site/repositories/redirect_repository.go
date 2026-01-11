package repositories

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/site/models"
)

// RedirectRepository handles database operations for redirects
type RedirectRepository struct {
	*BaseRepository
}

// NewRedirectRepository creates a new redirect repository
func NewRedirectRepository(db *gorm.DB) *RedirectRepository {
	return &RedirectRepository{
		BaseRepository: NewBaseRepository(db),
	}
}

// Create creates a new redirect
func (r *RedirectRepository) Create(ctx context.Context, redirect *models.Redirect) error {
	return r.db.WithContext(ctx).Create(redirect).Error
}

// FindByID finds a redirect by ID
func (r *RedirectRepository) FindByID(ctx context.Context, id string) (*models.Redirect, error) {
	var redirect models.Redirect
	err := r.db.WithContext(ctx).First(&redirect, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrRedirectNotFound
		}

		return nil, err
	}

	return &redirect, nil
}

// FindBySite finds all redirects for a site
func (r *RedirectRepository) FindBySite(ctx context.Context, siteID string) ([]models.Redirect, error) {
	var redirects []models.Redirect
	err := r.db.WithContext(ctx).
		Where("site_id = ?", siteID).
		Order("created_at DESC").
		Find(&redirects).Error

	return redirects, err
}

// Update updates a redirect
func (r *RedirectRepository) Update(ctx context.Context, redirect *models.Redirect) error {
	return r.db.WithContext(ctx).Save(redirect).Error
}

// Delete deletes a redirect
func (r *RedirectRepository) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&models.Redirect{}, "id = ?", id).Error
}
