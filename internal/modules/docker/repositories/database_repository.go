package repositories

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/docker/models"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/repository"
)

// DatabaseRepository handles persistence for managed-database rows.
// Same defence-in-depth shape as ApplicationRepository.
type DatabaseRepository struct {
	repository.Base[models.Database]
}

func NewDatabaseRepository(db *gorm.DB) *DatabaseRepository {
	return &DatabaseRepository{Base: repository.NewBase[models.Database](db)}
}

func (r *DatabaseRepository) FindByIDAndTeamServer(
	ctx context.Context, id, teamID, serverID string,
) (*models.Database, error) {
	var d models.Database
	err := r.DB.WithContext(ctx).
		Where("id = ? AND team_id = ? AND server_id = ?", id, teamID, serverID).
		First(&d).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fiberutil.NotFound()
		}
		return nil, err
	}
	return &d, nil
}

func (r *DatabaseRepository) ListForProject(
	ctx context.Context, teamID, projectID string,
) ([]models.Database, error) {
	var rows []models.Database
	err := r.DB.WithContext(ctx).
		Where("team_id = ? AND project_id = ?", teamID, projectID).
		Order("created_at DESC").
		Find(&rows).Error
	return rows, err
}

func (r *DatabaseRepository) ExistsByNameInProject(
	ctx context.Context, projectID, name, excludeID string,
) (bool, error) {
	q := r.DB.WithContext(ctx).
		Model(&models.Database{}).
		Where("project_id = ? AND name = ?", projectID, name)
	if excludeID != "" {
		q = q.Where("id <> ?", excludeID)
	}
	var count int64
	if err := q.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}
