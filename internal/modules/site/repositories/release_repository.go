package repositories

import (
	"context"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/site/models"
	"github.com/kkz6/launch-go/internal/pkg/repository"
)

// ReleaseRepository handles database operations for releases.
// Embeds repository.Base[T] for common CRUD operations.
type ReleaseRepository struct {
	repository.Base[models.Release]
}

// NewReleaseRepository creates a new release repository
func NewReleaseRepository(db *gorm.DB) *ReleaseRepository {
	return &ReleaseRepository{
		Base: repository.NewBase[models.Release](db),
	}
}

// FindByID finds a release by ID with custom error.
func (r *ReleaseRepository) FindByID(ctx context.Context, id string) (*models.Release, error) {
	release, err := r.Base.FindByID(ctx, id)
	if err != nil {
		if repository.IsNotFound(err) {
			return nil, ErrReleaseNotFound
		}
		return nil, err
	}
	return release, nil
}

// FindBySite finds all releases for a site
func (r *ReleaseRepository) FindBySite(ctx context.Context, siteID string) ([]models.Release, error) {
	var releases []models.Release
	err := r.DB.WithContext(ctx).
		Where("site_id = ?", siteID).
		Order("created_at DESC").
		Find(&releases).Error

	return releases, err
}

// CountBySite counts releases for a site
func (r *ReleaseRepository) CountBySite(ctx context.Context, siteID string) (int64, error) {
	var count int64
	err := r.DB.WithContext(ctx).
		Model(&models.Release{}).
		Where("site_id = ?", siteID).
		Count(&count).Error

	return count, err
}

// DeleteOld deletes old releases beyond the retention limit
func (r *ReleaseRepository) DeleteOld(ctx context.Context, siteID string, keepCount int) error {
	// Get IDs of releases to keep
	var keepIDs []string
	if err := r.DB.WithContext(ctx).
		Model(&models.Release{}).
		Select("id").
		Where("site_id = ?", siteID).
		Order("created_at DESC").
		Limit(keepCount).
		Pluck("id", &keepIDs).Error; err != nil {
		return err
	}

	if len(keepIDs) == 0 {
		return nil
	}

	// Delete releases not in the keep list
	return r.DB.WithContext(ctx).
		Where("site_id = ? AND id NOT IN ?", siteID, keepIDs).
		Delete(&models.Release{}).Error
}

// Note: The following methods are inherited from repository.Base[T]:
// - Create(ctx, entity) error
// - Delete(ctx, id) error
// - UpdateFields(ctx, id, fields) error
// - Transaction(ctx, fn) error
