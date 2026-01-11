package repositories

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/site/models"
)

// CommandRepository handles database operations for commands
type CommandRepository struct {
	*BaseRepository
}

// NewCommandRepository creates a new command repository
func NewCommandRepository(db *gorm.DB) *CommandRepository {
	return &CommandRepository{
		BaseRepository: NewBaseRepository(db),
	}
}

// Create creates a new command
func (r *CommandRepository) Create(ctx context.Context, cmd *models.Command) error {
	return r.db.WithContext(ctx).Create(cmd).Error
}

// FindByID finds a command by ID
func (r *CommandRepository) FindByID(ctx context.Context, id string) (*models.Command, error) {
	var cmd models.Command
	err := r.db.WithContext(ctx).First(&cmd, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrCommandNotFound
		}

		return nil, err
	}

	return &cmd, nil
}

// FindBySite finds all commands for a site
func (r *CommandRepository) FindBySite(ctx context.Context, siteID string) ([]models.Command, error) {
	var cmds []models.Command
	err := r.db.WithContext(ctx).
		Where("site_id = ?", siteID).
		Order("created_at DESC").
		Find(&cmds).Error

	return cmds, err
}

// Update updates a command
func (r *CommandRepository) Update(ctx context.Context, cmd *models.Command) error {
	return r.db.WithContext(ctx).Save(cmd).Error
}

// Delete deletes a command
func (r *CommandRepository) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&models.Command{}, "id = ?", id).Error
}
