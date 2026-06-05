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
	return repository.FindOne[models.Command](ctx, r.DB,
		repository.Preload("User"),
		repository.WithID(id),
	)
}

// FindBySite finds all commands for a site
func (r *CommandRepository) FindBySite(ctx context.Context, siteID string) ([]models.Command, error) {
	return repository.FindAll[models.Command](ctx, r.DB,
		repository.Preload("User"),
		repository.WithSiteID(siteID),
		repository.OrderByCreatedDesc(),
	)
}

// DeleteBySite deletes all commands for a site
func (r *CommandRepository) DeleteBySite(ctx context.Context, siteID string) error {
	return r.DB.WithContext(ctx).
		Where("site_id = ?", siteID).
		Delete(&models.Command{}).Error
}
