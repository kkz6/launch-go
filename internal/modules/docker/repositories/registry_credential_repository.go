package repositories

import (
	"context"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/docker/models"
	"github.com/kkz6/launch-go/internal/pkg/repository"
)

// RegistryCredentialRepository handles persistence for saved docker
// registry logins. Per-team scoping is enforced at the service layer;
// the repo just exposes the CRUD primitives + a "list by team" helper.
type RegistryCredentialRepository struct {
	repository.Base[models.RegistryCredential]
}

func NewRegistryCredentialRepository(db *gorm.DB) *RegistryCredentialRepository {
	return &RegistryCredentialRepository{
		Base: repository.NewBase[models.RegistryCredential](db),
	}
}

// FindByID returns a credential by ID, mapping NotFound to a HTTP 404.
// The caller is responsible for re-checking team scope — the FK on
// the table doesn't guard against cross-team reads.
func (r *RegistryCredentialRepository) FindByID(
	ctx context.Context, id string,
) (*models.RegistryCredential, error) {
	return repository.FindOne[models.RegistryCredential](ctx, r.DB,
		repository.WithID(id),
	)
}

// FindByIDForTeam combines FindByID + team-scope check. Used on the
// hot path (picker resolution at workload create) so we don't leak
// existence of another team's credentials.
func (r *RegistryCredentialRepository) FindByIDForTeam(
	ctx context.Context, id, teamID string,
) (*models.RegistryCredential, error) {
	return repository.FindOne[models.RegistryCredential](ctx, r.DB,
		repository.WithID(id),
		repository.WithTeamID(teamID),
	)
}

// ListForTeam returns every live credential the team owns. Ordered
// by name so the picker dropdown is stable across reloads. No
// pagination: a team realistically has a handful of registries.
func (r *RegistryCredentialRepository) ListForTeam(
	ctx context.Context, teamID string,
) ([]models.RegistryCredential, error) {
	return repository.FindAll[models.RegistryCredential](ctx, r.DB,
		repository.WithTeamID(teamID),
		repository.OrderBy("name", "asc"),
	)
}

// FindManyForTeam loads a specific set of credentials, scoped to the
// team. Used by the compose deploy job to fetch all attached
// credentials in one query before doing `docker login` for each.
// Returns the rows in arbitrary order (caller doesn't care).
func (r *RegistryCredentialRepository) FindManyForTeam(
	ctx context.Context, ids []string, teamID string,
) ([]models.RegistryCredential, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var rows []models.RegistryCredential
	err := r.DB.WithContext(ctx).
		Where("id IN ? AND team_id = ?", ids, teamID).
		Find(&rows).Error
	return rows, err
}

// ExistsByName returns true if a live credential with this name is
// already attached to the team. Migration also has a unique index on
// (team_id, name, deleted_at) — this check just lets the service
// return a friendly 409 instead of a generic 500.
func (r *RegistryCredentialRepository) ExistsByName(
	ctx context.Context, teamID, name, excludeID string,
) (bool, error) {
	q := r.DB.WithContext(ctx).
		Model(&models.RegistryCredential{}).
		Where("team_id = ? AND name = ?", teamID, name)
	if excludeID != "" {
		q = q.Where("id <> ?", excludeID)
	}
	var count int64
	if err := q.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}
