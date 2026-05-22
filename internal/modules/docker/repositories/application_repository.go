package repositories

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/docker/models"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/repository"
)

// ApplicationRepository handles persistence for docker applications.
type ApplicationRepository struct {
	repository.Base[models.Application]
}

// NewApplicationRepository wires the base repository for applications.
func NewApplicationRepository(db *gorm.DB) *ApplicationRepository {
	return &ApplicationRepository{Base: repository.NewBase[models.Application](db)}
}

// FindByIDAndTeamServer returns the application iff the (id, team, server)
// triple matches. Same tenancy-defence as ProjectRepository — wrong
// team/server returns NotFound rather than leaking that an ID exists.
func (r *ApplicationRepository) FindByIDAndTeamServer(
	ctx context.Context, id, teamID, serverID string,
) (*models.Application, error) {
	var a models.Application
	err := r.DB.WithContext(ctx).
		Where("id = ? AND team_id = ? AND server_id = ?", id, teamID, serverID).
		First(&a).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fiberutil.NotFound()
		}
		return nil, err
	}
	return &a, nil
}

// ListForProject returns every live application belonging to a project,
// ordered with the most recently created first so brand-new apps land on
// top of the list. We scope by team_id too even though project_id already
// implies a team, because the API entrypoint always has both and a defence-
// in-depth match against future bugs is cheap.
func (r *ApplicationRepository) ListForProject(
	ctx context.Context, teamID, projectID string,
) ([]models.Application, error) {
	var apps []models.Application
	err := r.DB.WithContext(ctx).
		Where("team_id = ? AND project_id = ?", teamID, projectID).
		Order("created_at DESC").
		Find(&apps).Error
	return apps, err
}

// ExistsByNameInProject returns true if a live application with the given
// name exists in the project. Mirrors the rationale in
// ProjectRepository.ExistsByName — surfacing a 409 from the service is
// nicer than letting the DB unique index throw a 500.
func (r *ApplicationRepository) ExistsByNameInProject(
	ctx context.Context, projectID, name, excludeID string,
) (bool, error) {
	q := r.DB.WithContext(ctx).
		Model(&models.Application{}).
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
