package repositories

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/docker/models"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/repository"
)

// ProjectRepository handles persistence for docker projects.
type ProjectRepository struct {
	repository.Base[models.Project]
}

// NewProjectRepository wires the base repository for projects.
func NewProjectRepository(db *gorm.DB) *ProjectRepository {
	return &ProjectRepository{Base: repository.NewBase[models.Project](db)}
}

// FindByIDAndTeamServer finds a project scoped to a (team, server) pair so
// a malicious request can't fetch another team's project by guessing IDs.
func (r *ProjectRepository) FindByIDAndTeamServer(
	ctx context.Context, id, teamID, serverID string,
) (*models.Project, error) {
	var p models.Project
	err := r.DB.WithContext(ctx).
		Where("id = ? AND team_id = ? AND server_id = ?", id, teamID, serverID).
		First(&p).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fiberutil.NotFound()
		}
		return nil, err
	}
	r.fillCounts(ctx, &p)
	return &p, nil
}

// ListForServer returns all live projects for a server, populated with
// workload counts so the UI doesn't need to N+1 the cards.
func (r *ProjectRepository) ListForServer(
	ctx context.Context, teamID, serverID string,
) ([]models.Project, error) {
	var ps []models.Project
	err := r.DB.WithContext(ctx).
		Where("team_id = ? AND server_id = ?", teamID, serverID).
		Order("created_at DESC").
		Find(&ps).Error
	if err != nil {
		return nil, err
	}
	for i := range ps {
		r.fillCounts(ctx, &ps[i])
	}
	return ps, nil
}

// ExistsByName returns true if a non-deleted project with this name already
// exists on the server. Used to give a 409-style error in the service
// layer before hitting the DB's unique index (which would otherwise
// surface as a generic 500).
//
// `excludeID` lets the caller skip a specific row when checking during an
// update — pass "" when creating.
func (r *ProjectRepository) ExistsByName(
	ctx context.Context, serverID, name, excludeID string,
) (bool, error) {
	q := r.DB.WithContext(ctx).
		Model(&models.Project{}).
		Where("server_id = ? AND name = ?", serverID, name)
	if excludeID != "" {
		q = q.Where("id <> ?", excludeID)
	}
	var count int64
	if err := q.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// fillCounts populates ApplicationsCount/ComposesCount/DatabasesCount on
// the given project. Best-effort — errors are swallowed so a stale count
// never blocks the read path.
func (r *ProjectRepository) fillCounts(ctx context.Context, p *models.Project) {
	type result struct {
		Count int64
	}
	for _, t := range []struct {
		table string
		dst   *int64
	}{
		{"docker_applications", &p.ApplicationsCount},
		{"docker_composes", &p.ComposesCount},
		{"docker_databases", &p.DatabasesCount},
	} {
		var r0 result
		_ = r.DB.WithContext(ctx).
			Table(t.table).
			Where("project_id = ? AND deleted_at IS NULL", p.ID).
			Count(&r0.Count).Error
		*t.dst = r0.Count
	}
}
