package repositories

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/docker/models"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/repository"
)

// EnvVarRepository handles persistence for application env vars.
type EnvVarRepository struct {
	repository.Base[models.ApplicationEnvVar]
}

func NewEnvVarRepository(db *gorm.DB) *EnvVarRepository {
	return &EnvVarRepository{Base: repository.NewBase[models.ApplicationEnvVar](db)}
}

func (r *EnvVarRepository) FindByID(ctx context.Context, id string) (*models.ApplicationEnvVar, error) {
	var v models.ApplicationEnvVar
	err := r.DB.WithContext(ctx).Where("id = ?", id).First(&v).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fiberutil.NotFound()
		}
		return nil, err
	}
	return &v, nil
}

func (r *EnvVarRepository) ListForApplication(
	ctx context.Context, applicationID string,
) ([]models.ApplicationEnvVar, error) {
	var rows []models.ApplicationEnvVar
	err := r.DB.WithContext(ctx).
		Where("application_id = ?", applicationID).
		Order("`key` ASC").
		Find(&rows).Error
	return rows, err
}

// ExistsByKey reports whether a live env var with this key is already
// attached to the application. Used to convert the DB unique-violation
// into a friendly 409.
func (r *EnvVarRepository) ExistsByKey(
	ctx context.Context, applicationID, key, excludeID string,
) (bool, error) {
	q := r.DB.WithContext(ctx).
		Model(&models.ApplicationEnvVar{}).
		Where("application_id = ? AND `key` = ?", applicationID, key)
	if excludeID != "" {
		q = q.Where("id <> ?", excludeID)
	}
	var count int64
	if err := q.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}
