package repositories

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/dockerregistry/models"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/repository"
)

// CredentialRepository handles docker registry credential database operations.
type CredentialRepository struct {
	repository.Base[models.Credential]
}

// NewCredentialRepository constructs a new repository.
func NewCredentialRepository(db *gorm.DB) *CredentialRepository {
	return &CredentialRepository{
		Base: repository.NewBase[models.Credential](db),
	}
}

// FindByIDAndTeam returns a credential scoped to the team, or NotFound.
func (r *CredentialRepository) FindByIDAndTeam(ctx context.Context, id, teamID string) (*models.Credential, error) {
	var c models.Credential
	err := r.DB.WithContext(ctx).
		Where("id = ? AND team_id = ?", id, teamID).
		First(&c).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fiberutil.NotFound()
		}
		return nil, err
	}
	return &c, nil
}

// FindByTeam returns every credential for a team, ordered by name.
func (r *CredentialRepository) FindByTeam(ctx context.Context, teamID string) ([]models.Credential, error) {
	var out []models.Credential
	err := r.DB.WithContext(ctx).
		Where("team_id = ?", teamID).
		Order("name ASC").
		Find(&out).Error
	return out, err
}

// FindByNameAndTeam returns a credential by name within a team, or nil if
// none exists. Used to detect duplicates before insert/rename.
func (r *CredentialRepository) FindByNameAndTeam(ctx context.Context, name, teamID string) (*models.Credential, error) {
	var c models.Credential
	err := r.DB.WithContext(ctx).
		Where("team_id = ? AND name = ?", teamID, name).
		First(&c).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &c, nil
}

// Update applies partial column updates to a credential.
func (r *CredentialRepository) Update(ctx context.Context, id string, updates map[string]any) error {
	result := r.DB.WithContext(ctx).
		Model(&models.Credential{}).
		Where("id = ?", id).
		Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fiberutil.NotFound()
	}
	return nil
}

// Delete removes a credential row by ID.
func (r *CredentialRepository) Delete(ctx context.Context, id string) error {
	return r.DB.WithContext(ctx).Delete(&models.Credential{}, "id = ?", id).Error
}
