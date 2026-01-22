package contracts

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/billing/enums"
	"github.com/kkz6/launch-go/internal/modules/billing/models"
)

// SubscriptionRepository defines the interface for subscription data operations
type SubscriptionRepository interface {
	Create(ctx context.Context, subscription *models.Subscription) error
	FindByID(ctx context.Context, id string) (*models.Subscription, error)
	FindByLemonSqueezyID(ctx context.Context, lemonSqueezyID string) (*models.Subscription, error)
	FindByTeam(ctx context.Context, teamID string) ([]models.Subscription, error)
	FindActiveByTeam(ctx context.Context, teamID string) (*models.Subscription, error)
	Update(ctx context.Context, subscription *models.Subscription) error
	UpdateStatus(ctx context.Context, id string, status enums.SubscriptionStatus) error
	UpdateStatusByLemonSqueezyID(ctx context.Context, lemonSqueezyID string, status enums.SubscriptionStatus) error
	UpdateFields(ctx context.Context, id string, fields map[string]interface{}) error
	Delete(ctx context.Context, id string) error
	CountActiveByTeam(ctx context.Context, teamID string) (int64, error)
	IsTeamSubscribed(ctx context.Context, teamID string) (bool, error)
	GetTeamSubscriptionInfo(ctx context.Context, teamID string) (*models.TeamSubscription, error)
}

// OrderRepository defines the interface for order data operations
type OrderRepository interface {
	Create(ctx context.Context, order *models.Order) error
	FindByID(ctx context.Context, id string) (*models.Order, error)
	FindByLemonSqueezyID(ctx context.Context, lemonSqueezyID string) (*models.Order, error)
	FindByTeam(ctx context.Context, teamID string) ([]models.Order, error)
	FindBySubscription(ctx context.Context, subscriptionID string) ([]models.Order, error)
	Update(ctx context.Context, order *models.Order) error
	UpdateStatus(ctx context.Context, id string, status enums.OrderStatus) error
}

// WebhookEventRepository defines the interface for webhook event data operations
type WebhookEventRepository interface {
	Create(ctx context.Context, event *models.WebhookEvent) error
	FindByID(ctx context.Context, id string) (*models.WebhookEvent, error)
	FindUnprocessed(ctx context.Context, maxRetries int) ([]models.WebhookEvent, error)
	Update(ctx context.Context, event *models.WebhookEvent) error
	MarkProcessed(ctx context.Context, id string) error
	MarkFailed(ctx context.Context, id string, errMsg string) error
	DeleteOldProcessed(ctx context.Context, olderThan time.Duration) error
}

// RepositoryRegistry provides access to all billing repositories
type RepositoryRegistry interface {
	Subscription() SubscriptionRepository
	Order() OrderRepository
	WebhookEvent() WebhookEventRepository
	DB() *gorm.DB
}
