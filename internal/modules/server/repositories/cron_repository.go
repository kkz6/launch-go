package repositories

import (
	"context"
	"errors"

	"gorm.io/gorm"

	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"

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
			return nil, fiberutil.NotFound()
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
			return nil, fiberutil.NotFound()
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
			return nil, fiberutil.NotFound()
		}
		return nil, err
	}
	return cron, nil
}

// CountBySite counts cron jobs associated with a site
func (r *CronRepository) CountBySite(ctx context.Context, siteID string) (int64, error) {
	var count int64
	err := r.DB.WithContext(ctx).
		Model(&models.Cron{}).
		Where("site_id = ?", siteID).
		Count(&count).Error

	return count, err
}

// Note: MarkAsInstalled, MarkInstallationFailed and MarkUninstallationFailed are inherited from repository.Installable
