package repositories

import (
	"context"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/docker/models"
	"github.com/kkz6/launch-go/internal/pkg/repository"
)

// ComposeBuildSecretRepository is the compose-stack mirror of
// BuildSecretRepository. Same shape, different owner FK (compose_id
// instead of application_id).
type ComposeBuildSecretRepository struct {
	repository.Base[models.ComposeBuildSecret]
}

func NewComposeBuildSecretRepository(db *gorm.DB) *ComposeBuildSecretRepository {
	return &ComposeBuildSecretRepository{Base: repository.NewBase[models.ComposeBuildSecret](db)}
}

func (r *ComposeBuildSecretRepository) FindByID(ctx context.Context, id string) (*models.ComposeBuildSecret, error) {
	return repository.FindOne[models.ComposeBuildSecret](ctx, r.DB,
		repository.WithID(id),
	)
}

func (r *ComposeBuildSecretRepository) ListForCompose(
	ctx context.Context, composeID string,
) ([]models.ComposeBuildSecret, error) {
	var rows []models.ComposeBuildSecret
	err := r.DB.WithContext(ctx).
		Where("compose_id = ?", composeID).
		Order("name ASC").
		Find(&rows).Error
	return rows, err
}

func (r *ComposeBuildSecretRepository) ExistsByName(
	ctx context.Context, composeID, name, excludeID string,
) (bool, error) {
	q := r.DB.WithContext(ctx).
		Model(&models.ComposeBuildSecret{}).
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
