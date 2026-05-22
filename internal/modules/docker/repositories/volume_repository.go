package repositories

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/docker/models"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/repository"
)

// VolumeRepository handles persistence for application volumes.
type VolumeRepository struct {
	repository.Base[models.ApplicationVolume]
}

func NewVolumeRepository(db *gorm.DB) *VolumeRepository {
	return &VolumeRepository{Base: repository.NewBase[models.ApplicationVolume](db)}
}

func (r *VolumeRepository) FindByID(ctx context.Context, id string) (*models.ApplicationVolume, error) {
	var v models.ApplicationVolume
	err := r.DB.WithContext(ctx).Where("id = ?", id).First(&v).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fiberutil.NotFound()
		}
		return nil, err
	}
	return &v, nil
}

func (r *VolumeRepository) ListForApplication(
	ctx context.Context, applicationID string,
) ([]models.ApplicationVolume, error) {
	var rows []models.ApplicationVolume
	err := r.DB.WithContext(ctx).
		Where("application_id = ?", applicationID).
		Order("name ASC").
		Find(&rows).Error
	return rows, err
}

// ExistsByName reports whether a live volume with this name is already
// attached to the application.
func (r *VolumeRepository) ExistsByName(
	ctx context.Context, applicationID, name, excludeID string,
) (bool, error) {
	q := r.DB.WithContext(ctx).
		Model(&models.ApplicationVolume{}).
		Where("application_id = ? AND name = ?", applicationID, name)
	if excludeID != "" {
		q = q.Where("id <> ?", excludeID)
	}
	var count int64
	if err := q.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}
