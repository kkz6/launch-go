package repositories

import (
	"context"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/site/models"
	"github.com/kkz6/launch-go/internal/pkg/repository"
)

// CommandRepository handles database operations for commands.
type CommandRepository struct {
	repository.Base[models.Command]
}

// NewCommandRepository creates a new command repository
func NewCommandRepository(db *gorm.DB) *CommandRepository {
	return &CommandRepository{
		Base: repository.NewBase[models.Command](db),
	}
}

// FindByID finds a command by ID with custom error.
func (r *CommandRepository) FindByID(ctx context.Context, id string) (*models.Command, error) {
	var cmd models.Command
	err := r.DB.WithContext(ctx).
		Preload("User").
		Where("id = ?", id).
		First(&cmd).Error
	if err != nil {
		if repository.IsNotFound(err) {
			return nil, ErrCommandNotFound
		}
		return nil, err
	}
	return &cmd, nil
}

// FindBySite finds all commands for a site
func (r *CommandRepository) FindBySite(ctx context.Context, siteID string) ([]models.Command, error) {
	var cmds []models.Command
	err := r.DB.WithContext(ctx).
		Preload("User").
		Where("site_id = ?", siteID).
		Order("created_at DESC").
		Find(&cmds).Error

	return cmds, err
}

// DeleteBySite deletes all commands for a site
func (r *CommandRepository) DeleteBySite(ctx context.Context, siteID string) error {
	return r.DB.WithContext(ctx).
		Where("site_id = ?", siteID).
		Delete(&models.Command{}).Error
}
