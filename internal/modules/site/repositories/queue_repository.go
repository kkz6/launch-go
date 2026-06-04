package repositories

import (
	"context"
	"time"

	"gorm.io/gorm"

	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"

	"github.com/kkz6/launch-go/internal/modules/site/models"
	"github.com/kkz6/launch-go/internal/pkg/repository"
)

// QueueRepository handles database operations for queues.
type QueueRepository struct {
	repository.Installable[models.Queue]
}

// NewQueueRepository creates a new queue repository
func NewQueueRepository(db *gorm.DB) *QueueRepository {
	return &QueueRepository{
		Installable: repository.NewInstallable[models.Queue](db),
	}
}

// FindByID finds a queue by ID with custom error.
func (r *QueueRepository) FindByID(ctx context.Context, id string) (*models.Queue, error) {
	queue, err := r.Installable.FindByID(ctx, id)
	if err != nil {
		if repository.IsNotFound(err) {
			return nil, fiberutil.NotFound()
		}
		return nil, err
	}
	return queue, nil
}

// FindByIDAndSite finds a queue by ID and site ID
func (r *QueueRepository) FindByIDAndSite(ctx context.Context, id, siteID string) (*models.Queue, error) {
	return repository.FindOne[models.Queue](ctx, r.DB,
		repository.WithID(id),
		repository.WithSiteID(siteID),
	)
}

// FindBySite finds all queues for a site
func (r *QueueRepository) FindBySite(ctx context.Context, siteID string) ([]models.Queue, error) {
	return repository.FindAll[models.Queue](ctx, r.DB,
		repository.WithSiteID(siteID),
		repository.OrderByCreatedDesc(),
	)
}

// CountBySite counts queues for a site
func (r *QueueRepository) CountBySite(ctx context.Context, siteID string) (int64, error) {
	var count int64
	err := r.DB.WithContext(ctx).
		Model(&models.Queue{}).
		Where("site_id = ?", siteID).
		Count(&count).Error

	return count, err
}

// UpdateLastStatusCheckBySite updates the last_status_check for all queues of a site
func (r *QueueRepository) UpdateLastStatusCheckBySite(ctx context.Context, siteID string, t time.Time) error {
	return r.DB.WithContext(ctx).
		Model(&models.Queue{}).
		Where("site_id = ?", siteID).
		Update("last_status_check", t).Error
}
