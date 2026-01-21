package repositories

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/pkg/repository"
)

// CronRepository handles cron job database operations
type CronRepository struct {
	repository.Installable[models.Cron]
}

// NewCronRepository creates a new CronRepository instance
func NewCronRepository(db *gorm.DB) *CronRepository {
	return &CronRepository{
		Installable: repository.NewInstallable[models.Cron](db),
	}
}

// FindByID finds a cron job by ID (override to return specific error)
func (r *CronRepository) FindByID(ctx context.Context, id string) (*models.Cron, error) {
	cron, err := r.Installable.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrCronNotFound
		}
		return nil, err
	}
	return cron, nil
}

// FindByIDWithServer finds a cron job by ID with the Server relation preloaded (override to return specific error)
func (r *CronRepository) FindByIDWithServer(ctx context.Context, id string) (*models.Cron, error) {
	cron, err := r.Installable.FindByIDWithServer(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrCronNotFound
		}
		return nil, err
	}
	return cron, nil
}

// FindByIDAndServer finds a cron job by ID and server ID (override to return specific error)
func (r *CronRepository) FindByIDAndServer(ctx context.Context, id, serverID string) (*models.Cron, error) {
	cron, err := r.Installable.FindByIDAndServer(ctx, id, serverID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrCronNotFound
		}
		return nil, err
	}
	return cron, nil
}

// MarkInstalled marks a cron job as installed (alias for MarkAsInstalled)
func (r *CronRepository) MarkInstalled(ctx context.Context, id string) error {
	return r.MarkAsInstalled(ctx, id)
}
