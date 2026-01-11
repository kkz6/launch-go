package models

import (
	"time"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/billing/enums"
	"github.com/kkz6/launch-go/internal/pkg/utils"
)

// WebhookEvent represents a webhook event received from LemonSqueezy
type WebhookEvent struct {
	ID          string               `gorm:"primaryKey;size:26" json:"id"`
	EventName   enums.WebhookEventType `gorm:"size:100;not null;index" json:"event_name"`
	Payload     string               `gorm:"type:text;not null" json:"payload"`
	Signature   string               `gorm:"size:255;not null" json:"signature"`
	Processed   bool                 `gorm:"default:false" json:"processed"`
	ProcessedAt *time.Time           `json:"processed_at,omitempty"`
	Error       *string              `gorm:"type:text" json:"error,omitempty"`
	RetryCount  int                  `gorm:"default:0" json:"retry_count"`
	CreatedAt   time.Time            `json:"created_at"`
	UpdatedAt   time.Time            `json:"updated_at"`
}

// BeforeCreate hook to generate ULID
func (w *WebhookEvent) BeforeCreate(tx *gorm.DB) error {
	if w.ID == "" {
		w.ID = utils.NewULID()
	}

	return nil
}

// MarkProcessed marks the webhook event as processed
func (w *WebhookEvent) MarkProcessed() {
	w.Processed = true
	now := time.Now()
	w.ProcessedAt = &now
}

// MarkFailed marks the webhook event as failed with an error
func (w *WebhookEvent) MarkFailed(err string) {
	w.Error = &err
	w.RetryCount++
}

// CanRetry checks if the webhook event can be retried
func (w *WebhookEvent) CanRetry(maxRetries int) bool {
	return !w.Processed && w.RetryCount < maxRetries
}
