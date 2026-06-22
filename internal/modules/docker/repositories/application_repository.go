package repositories

import (
	"context"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/docker/models"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
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

// UpdateStatus is the single-column write used by lifecycle
// transitions (deploy, delete, stop). Goes through Model+Update so
// gorm doesn't bump every other column — keeps a delete-status
// flip from accidentally touching, say, last_deployed_at.
func (r *ApplicationRepository) UpdateStatus(
	ctx context.Context, id string, status string,
) error {
	return r.DB.WithContext(ctx).
		Model(&models.Application{}).
		Where("id = ?", id).
		Update("status", status).Error
}

// UpdateSourceConfig writes only the source_config column. Used by the
// build-secret sync tracking to flip gha_pending_changes without
// touching other fields (same single-column discipline as
// UpdateStatus).
func (r *ApplicationRepository) UpdateSourceConfig(
	ctx context.Context, id string, cfg dbtype.JSONMap,
) error {
	return r.DB.WithContext(ctx).
		Model(&models.Application{}).
		Where("id = ?", id).
		Update("source_config", cfg).Error
}

// FindByIDAndTeamServer returns the application iff the (id, team, server)
// triple matches. Same tenancy-defence as ProjectRepository — wrong
// team/server returns NotFound rather than leaking that an ID exists.
func (r *ApplicationRepository) FindByIDAndTeamServer(
	ctx context.Context, id, teamID, serverID string,
) (*models.Application, error) {
	return repository.FindOne[models.Application](ctx, r.DB,
		repository.WithID(id),
		repository.WithTeamID(teamID),
		repository.WithServerID(serverID),
	)
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
