package repositories

import (
	"context"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/site/models"
	"github.com/kkz6/launch-go/internal/pkg/repository"
)

// CommandRepository handles database operations for commands.
// Embeds repository.Base[T] for common CRUD operations.
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
	cmd, err := r.Base.FindByID(ctx, id)
	if err != nil {
		if repository.IsNotFound(err) {
			return nil, ErrCommandNotFound
		}
		return nil, err
	}
	return cmd, nil
}

// FindBySite finds all commands for a site
func (r *CommandRepository) FindBySite(ctx context.Context, siteID string) ([]models.Command, error) {
	var cmds []models.Command
	err := r.DB.WithContext(ctx).
		Where("site_id = ?", siteID).
		Order("created_at DESC").
		Find(&cmds).Error

	return cmds, err
}

// Note: The following methods are inherited from repository.Base[T]:
// - Create(ctx, entity) error
// - Update(ctx, entity) error
// - Delete(ctx, id) error
// - UpdateFields(ctx, id, fields) error
// - Transaction(ctx, fn) error
