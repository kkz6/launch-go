package repositories

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/docker/models"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/repository"
)

// ComposeRepository handles persistence for compose stacks. Mirrors
// ApplicationRepository — every query is scoped through (team, server)
// so URL-tampering across teams returns NotFound, never 403.
type ComposeRepository struct {
	repository.Base[models.Compose]
}

// NewComposeRepository wires the base repository for compose stacks.
func NewComposeRepository(db *gorm.DB) *ComposeRepository {
	return &ComposeRepository{Base: repository.NewBase[models.Compose](db)}
}

// FindByIDAndTeamServer fetches a compose stack scoped to the (team,
// server) pair. Same defence-in-depth shape as
// ApplicationRepository.FindByIDAndTeamServer.
func (r *ComposeRepository) FindByIDAndTeamServer(
	ctx context.Context, id, teamID, serverID string,
) (*models.Compose, error) {
	var c models.Compose
	err := r.DB.WithContext(ctx).
		Where("id = ? AND team_id = ? AND server_id = ?", id, teamID, serverID).
		First(&c).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fiberutil.NotFound()
		}
		return nil, err
	}
	return &c, nil
}

// ListForProject returns compose stacks in a project, newest first.
func (r *ComposeRepository) ListForProject(
	ctx context.Context, teamID, projectID string,
) ([]models.Compose, error) {
	var rows []models.Compose
	err := r.DB.WithContext(ctx).
		Where("team_id = ? AND project_id = ?", teamID, projectID).
		Order("created_at DESC").
		Find(&rows).Error
	return rows, err
}

// ExistsByNameInProject returns true if a live compose with this name
// exists in the project. Used to give a 409 before the unique index
// throws a generic 500.
func (r *ComposeRepository) ExistsByNameInProject(
	ctx context.Context, projectID, name, excludeID string,
) (bool, error) {
	q := r.DB.WithContext(ctx).
		Model(&models.Compose{}).
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
