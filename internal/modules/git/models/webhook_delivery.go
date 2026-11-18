package models

import (
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	"time"
)

// Receipts fence redelivery; intents make fanout retryable after queue failures.
type GitHubWebhookDelivery struct {
	ID          string `gorm:"primaryKey;size:128"`
	Event       string
	PayloadHash string
	ProcessedAt *time.Time
	CreatedAt   time.Time
}

type GitHubWebhookIntent struct {
	ID           string `gorm:"primaryKey;size:128"`
	DeliveryID   string `gorm:"index"`
	SiteID       string
	TaskType     string
	Payload      dbtype.JSONMap `gorm:"type:json"`
	DispatchedAt *time.Time
	CreatedAt    time.Time
}
