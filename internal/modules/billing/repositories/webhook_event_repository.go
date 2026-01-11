package repositories

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/billing/models"
)

// WebhookEventRepository handles database operations for webhook events
type WebhookEventRepository struct {
	db *gorm.DB
}

// NewWebhookEventRepository creates a new webhook event repository
func NewWebhookEventRepository(db *gorm.DB) *WebhookEventRepository {
	return &WebhookEventRepository{db: db}
}

// Create creates a new webhook event
func (r *WebhookEventRepository) Create(ctx context.Context, event *models.WebhookEvent) error {
	return r.db.WithContext(ctx).Create(event).Error
}

// FindByID finds a webhook event by ID
func (r *WebhookEventRepository) FindByID(ctx context.Context, id string) (*models.WebhookEvent, error) {
	var event models.WebhookEvent
	err := r.db.WithContext(ctx).First(&event, "id = ?", id).Error
	if err != nil {
		return nil, err
	}

	return &event, nil
}

// FindUnprocessed finds all unprocessed webhook events
func (r *WebhookEventRepository) FindUnprocessed(ctx context.Context, maxRetries int) ([]models.WebhookEvent, error) {
	var events []models.WebhookEvent
	err := r.db.WithContext(ctx).
		Where("processed = ?", false).
		Where("retry_count < ?", maxRetries).
		Order("created_at ASC").
		Find(&events).Error

	return events, err
}

// Update updates a webhook event
func (r *WebhookEventRepository) Update(ctx context.Context, event *models.WebhookEvent) error {
	return r.db.WithContext(ctx).Save(event).Error
}

// MarkProcessed marks a webhook event as processed
func (r *WebhookEventRepository) MarkProcessed(ctx context.Context, id string) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&models.WebhookEvent{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"processed":    true,
			"processed_at": now,
		}).Error
}

// MarkFailed marks a webhook event as failed
func (r *WebhookEventRepository) MarkFailed(ctx context.Context, id string, errMsg string) error {
	return r.db.WithContext(ctx).
		Model(&models.WebhookEvent{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"error":       errMsg,
			"retry_count": gorm.Expr("retry_count + 1"),
		}).Error
}

// DeleteOldProcessed deletes processed webhook events older than the specified duration
func (r *WebhookEventRepository) DeleteOldProcessed(ctx context.Context, olderThan time.Duration) error {
	cutoff := time.Now().Add(-olderThan)
	return r.db.WithContext(ctx).
		Where("processed = ?", true).
		Where("processed_at < ?", cutoff).
		Delete(&models.WebhookEvent{}).Error
}
