package repositories

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/site/models"
)

// QueueRepository handles database operations for queues
type QueueRepository struct {
	*BaseRepository
}

// NewQueueRepository creates a new queue repository
func NewQueueRepository(db *gorm.DB) *QueueRepository {
	return &QueueRepository{
		BaseRepository: NewBaseRepository(db),
	}
}

// Create creates a new queue
func (r *QueueRepository) Create(ctx context.Context, queue *models.Queue) error {
	return r.db.WithContext(ctx).Create(queue).Error
}

// FindByID finds a queue by ID
func (r *QueueRepository) FindByID(ctx context.Context, id string) (*models.Queue, error) {
	var queue models.Queue
	err := r.db.WithContext(ctx).First(&queue, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrQueueNotFound
		}

		return nil, err
	}

	return &queue, nil
}

// FindByIDAndSite finds a queue by ID and site ID
func (r *QueueRepository) FindByIDAndSite(ctx context.Context, id, siteID string) (*models.Queue, error) {
	var queue models.Queue
	err := r.db.WithContext(ctx).
		First(&queue, "id = ? AND site_id = ?", id, siteID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrQueueNotFound
		}

		return nil, err
	}

	return &queue, nil
}

// FindBySite finds all queues for a site
func (r *QueueRepository) FindBySite(ctx context.Context, siteID string) ([]models.Queue, error) {
	var queues []models.Queue
	err := r.db.WithContext(ctx).
		Where("site_id = ?", siteID).
		Order("created_at DESC").
		Find(&queues).Error

	return queues, err
}

// FindByServer finds all queues for a server
func (r *QueueRepository) FindByServer(ctx context.Context, serverID string) ([]models.Queue, error) {
	var queues []models.Queue
	err := r.db.WithContext(ctx).
		Where("server_id = ?", serverID).
		Order("created_at DESC").
		Find(&queues).Error

	return queues, err
}

// CountBySite counts queues for a site
func (r *QueueRepository) CountBySite(ctx context.Context, siteID string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&models.Queue{}).
		Where("site_id = ?", siteID).
		Count(&count).Error

	return count, err
}

// Update updates a queue
func (r *QueueRepository) Update(ctx context.Context, queue *models.Queue) error {
	return r.db.WithContext(ctx).Save(queue).Error
}

// Delete deletes a queue
func (r *QueueRepository) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&models.Queue{}, "id = ?", id).Error
}
