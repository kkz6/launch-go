package repositories

import (
	"context"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/script/models"
	"github.com/kkz6/launch-go/internal/pkg/repository"
)

// ScriptRepository handles database operations for scripts
type ScriptRepository struct {
	repository.Base[models.Script]
}

// NewScriptRepository creates a new script repository
func NewScriptRepository(db *gorm.DB) *ScriptRepository {
	return &ScriptRepository{
		Base: repository.NewBase[models.Script](db),
	}
}

// FindByID finds a script by ID
func (r *ScriptRepository) FindByID(ctx context.Context, id string) (*models.Script, error) {
	script, err := r.Base.FindByID(ctx, id)
	if err != nil {
		if repository.IsNotFound(err) {
			return nil, ErrScriptNotFound
		}

		return nil, err
	}

	return script, nil
}

// FindByUserOrTeam finds scripts accessible to a user (personal + team shared)
func (r *ScriptRepository) FindByUserOrTeam(ctx context.Context, userID, teamID string) ([]models.Script, error) {
	var scripts []models.Script

	err := r.DB.WithContext(ctx).
		Where("user_id = ? OR team_id = ?", userID, teamID).
		Order("name ASC").
		Find(&scripts).Error

	return scripts, err
}

// FindByTeam finds all team-shared scripts
func (r *ScriptRepository) FindByTeam(ctx context.Context, teamID string) ([]models.Script, error) {
	var scripts []models.Script

	err := r.DB.WithContext(ctx).
		Where("team_id = ?", teamID).
		Order("name ASC").
		Find(&scripts).Error

	return scripts, err
}

// CanAccess checks if a user can access a script
func (r *ScriptRepository) CanAccess(ctx context.Context, scriptID, userID, teamID string) (bool, error) {
	var count int64

	err := r.DB.WithContext(ctx).
		Model(&models.Script{}).
		Where("id = ? AND (user_id = ? OR team_id = ?)", scriptID, userID, teamID).
		Count(&count).Error

	return count > 0, err
}
