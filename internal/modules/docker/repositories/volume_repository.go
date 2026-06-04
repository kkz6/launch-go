package repositories

import (
	"context"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/docker/models"
	"github.com/kkz6/launch-go/internal/pkg/repository"
)

// VolumeRepository handles persistence for docker volumes. The same
// row type backs both application-owned and compose-stack-owned
// mounts (polymorphic by `application_id` / `compose_id`); the
// per-owner List / ExistsByName methods scope queries to the right
// owner column so we never accidentally mix flavours in a response.
type VolumeRepository struct {
	repository.Base[models.ApplicationVolume]
}

func NewVolumeRepository(db *gorm.DB) *VolumeRepository {
	return &VolumeRepository{Base: repository.NewBase[models.ApplicationVolume](db)}
}

func (r *VolumeRepository) FindByID(ctx context.Context, id string) (*models.ApplicationVolume, error) {
	return repository.FindOne[models.ApplicationVolume](ctx, r.DB,
		repository.WithID(id),
	)
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

// ListForCompose returns every live volume attached to a compose
// stack — mirrors ListForApplication but filters on the compose
// owner column. Ordered by name so the UI gets a stable list.
func (r *VolumeRepository) ListForCompose(
	ctx context.Context, composeID string,
) ([]models.ApplicationVolume, error) {
	var rows []models.ApplicationVolume
	err := r.DB.WithContext(ctx).
		Where("compose_id = ?", composeID).
		Order("name ASC").
		Find(&rows).Error
	return rows, err
}

// ExistsByNameForCompose mirrors ExistsByName but scoped to a
// compose-owned row. Uniqueness is enforced per-owner — the same
// volume name can exist under a different application or compose
// stack without collision.
func (r *VolumeRepository) ExistsByNameForCompose(
	ctx context.Context, composeID, name, excludeID string,
) (bool, error) {
	q := r.DB.WithContext(ctx).
		Model(&models.ApplicationVolume{}).
		Where("compose_id = ? AND name = ?", composeID, name)
	if excludeID != "" {
		q = q.Where("id <> ?", excludeID)
	}
	var count int64
	if err := q.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}
