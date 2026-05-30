package repositories

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/docker/models"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/repository"
)

// BuildSecretRepository handles persistence for application build
// secrets — the build-time half of credentials customers ship into
// `docker build` via --mount=type=secret. Same CRUD shape as
// EnvVarRepository, just on a different table.
type BuildSecretRepository struct {
	repository.Base[models.ApplicationBuildSecret]
}

func NewBuildSecretRepository(db *gorm.DB) *BuildSecretRepository {
	return &BuildSecretRepository{Base: repository.NewBase[models.ApplicationBuildSecret](db)}
}

func (r *BuildSecretRepository) FindByID(ctx context.Context, id string) (*models.ApplicationBuildSecret, error) {
	var v models.ApplicationBuildSecret
	err := r.DB.WithContext(ctx).Where("id = ?", id).First(&v).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fiberutil.NotFound()
		}
		return nil, err
	}
	return &v, nil
}

func (r *BuildSecretRepository) ListForApplication(
	ctx context.Context, applicationID string,
) ([]models.ApplicationBuildSecret, error) {
	var rows []models.ApplicationBuildSecret
	err := r.DB.WithContext(ctx).
		Where("application_id = ?", applicationID).
		Order("name ASC").
		Find(&rows).Error
	return rows, err
}

// ExistsByName reports whether a live build secret with this name is
// already attached to the application. Used to convert the DB unique-
// violation into a friendly 409.
func (r *BuildSecretRepository) ExistsByName(
	ctx context.Context, applicationID, name, excludeID string,
) (bool, error) {
	q := r.DB.WithContext(ctx).
		Model(&models.ApplicationBuildSecret{}).
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
