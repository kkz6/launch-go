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

// FindByIDUnscoped includes soft-deleted rows. Called by the
// lifecycle worker on the `rm` path: the service soft-deletes the
// row first, then enqueues `rm`, so by the time the worker fetches
// the row the default scope hides it. Without this lookup the
// worker falls back to using the database ID (ULID) as the name in
// the container-name template, ends up running `docker rm
// launch-db-<project>-<ULID>` (which doesn't exist), and leaves the
// real container orphaned on the host.
func (r *DatabaseRepository) FindByIDUnscoped(
	ctx context.Context, id string,
) (*models.Database, error) {
	var d models.Database
	err := r.DB.WithContext(ctx).
		Unscoped().
		Where("id = ?", id).
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

// ListForServer returns every docker database on a single server,
// scoped to the caller's team. Used by the restore-target picker —
// the dialog needs a flat list across all projects (most "restore
// prod → staging" workflows put those rows in DIFFERENT projects).
//
// Sorted by name so the dropdown reads in a stable alphabetical
// order regardless of which user created the rows when.
func (r *DatabaseRepository) ListForServer(
	ctx context.Context, teamID, serverID string,
) ([]models.Database, error) {
	var rows []models.Database
	err := r.DB.WithContext(ctx).
		Where("team_id = ? AND server_id = ?", teamID, serverID).
		Order("name ASC").
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
