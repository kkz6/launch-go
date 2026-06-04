package repositories

import (
	"context"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/docker/models"
	"github.com/kkz6/launch-go/internal/pkg/repository"
)

// DatabaseEnvVarRepository handles persistence for env vars attached
// to a managed database container. Same shape as the application +
// project env-var repos — kept separate so each foreign-key column
// stays explicit in the query layer.
type DatabaseEnvVarRepository struct {
	repository.Base[models.DatabaseEnvVar]
}

func NewDatabaseEnvVarRepository(db *gorm.DB) *DatabaseEnvVarRepository {
	return &DatabaseEnvVarRepository{Base: repository.NewBase[models.DatabaseEnvVar](db)}
}

func (r *DatabaseEnvVarRepository) FindByID(ctx context.Context, id string) (*models.DatabaseEnvVar, error) {
	return repository.FindOne[models.DatabaseEnvVar](ctx, r.DB,
		repository.WithID(id),
	)
}

func (r *DatabaseEnvVarRepository) ListForDatabase(
	ctx context.Context, databaseID string,
) ([]models.DatabaseEnvVar, error) {
	var rows []models.DatabaseEnvVar
	err := r.DB.WithContext(ctx).
		Where("database_id = ?", databaseID).
		Order(`"key" ASC`).
		Find(&rows).Error
	return rows, err
}

func (r *DatabaseEnvVarRepository) ExistsByKey(
	ctx context.Context, databaseID, key, excludeID string,
) (bool, error) {
	q := r.DB.WithContext(ctx).
		Model(&models.DatabaseEnvVar{}).
		Where(`database_id = ? AND "key" = ?`, databaseID, key)
	if excludeID != "" {
		q = q.Where("id <> ?", excludeID)
	}
	var count int64
	if err := q.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}
