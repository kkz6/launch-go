package repositories

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/site/models"
)

// ReleaseRepository handles database operations for releases
type ReleaseRepository struct {
	*BaseRepository
}

// NewReleaseRepository creates a new release repository
func NewReleaseRepository(db *gorm.DB) *ReleaseRepository {
	return &ReleaseRepository{
		BaseRepository: NewBaseRepository(db),
	}
}

// Create creates a new release
func (r *ReleaseRepository) Create(ctx context.Context, release *models.Release) error {
	return r.db.WithContext(ctx).Create(release).Error
}

// FindByID finds a release by ID
func (r *ReleaseRepository) FindByID(ctx context.Context, id string) (*models.Release, error) {
	var release models.Release
	err := r.db.WithContext(ctx).First(&release, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrReleaseNotFound
		}

		return nil, err
	}

	return &release, nil
}

// FindBySite finds all releases for a site
func (r *ReleaseRepository) FindBySite(ctx context.Context, siteID string) ([]models.Release, error) {
	var releases []models.Release
	err := r.db.WithContext(ctx).
		Where("site_id = ?", siteID).
		Order("created_at DESC").
		Find(&releases).Error

	return releases, err
}

// CountBySite counts releases for a site
func (r *ReleaseRepository) CountBySite(ctx context.Context, siteID string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&models.Release{}).
		Where("site_id = ?", siteID).
		Count(&count).Error

	return count, err
}

// Delete deletes a release
func (r *ReleaseRepository) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&models.Release{}, "id = ?", id).Error
}

// DeleteOld deletes old releases beyond the retention limit
func (r *ReleaseRepository) DeleteOld(ctx context.Context, siteID string, keepCount int) error {
	// Get IDs of releases to keep
	var keepIDs []string
	if err := r.db.WithContext(ctx).
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
	return r.db.WithContext(ctx).
		Where("site_id = ? AND id NOT IN ?", siteID, keepIDs).
		Delete(&models.Release{}).Error
}
