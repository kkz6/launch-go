package repositories

import (
	"context"

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
	var queue models.Queue
	err := r.DB.WithContext(ctx).
		First(&queue, "id = ? AND site_id = ?", id, siteID).Error
	if err != nil {
		if repository.IsNotFound(err) {
			return nil, fiberutil.NotFound()
		}
		return nil, err
	}
	return &queue, nil
}

// FindBySite finds all queues for a site
func (r *QueueRepository) FindBySite(ctx context.Context, siteID string) ([]models.Queue, error) {
	var queues []models.Queue
	err := r.DB.WithContext(ctx).
		Where("site_id = ?", siteID).
		Order("created_at DESC").
		Find(&queues).Error

	return queues, err
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
