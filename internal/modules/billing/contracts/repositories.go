package contracts

import (
	"context"
	"time"

	"github.com/kkz6/launch-go/internal/modules/billing/enums"
	"github.com/kkz6/launch-go/internal/modules/billing/models"
)

// SubscriptionRepository defines the interface for subscription data operations
type SubscriptionRepository interface {
	CreateSubscription(ctx context.Context, subscription *models.Subscription) error
	FindSubscriptionByID(ctx context.Context, id string) (*models.Subscription, error)
	FindSubscriptionByLemonSqueezyID(ctx context.Context, lemonSqueezyID string) (*models.Subscription, error)
	FindSubscriptionsByTeam(ctx context.Context, teamID string) ([]models.Subscription, error)
	FindActiveSubscriptionByTeam(ctx context.Context, teamID string) (*models.Subscription, error)
	UpdateSubscription(ctx context.Context, subscription *models.Subscription) error
	UpdateSubscriptionStatus(ctx context.Context, id string, status enums.SubscriptionStatus) error
	UpdateSubscriptionFields(ctx context.Context, id string, fields map[string]interface{}) error
	DeleteSubscription(ctx context.Context, id string) error
	CountActiveSubscriptionsByTeam(ctx context.Context, teamID string) (int64, error)
	IsTeamSubscribed(ctx context.Context, teamID string) (bool, error)
	GetTeamSubscriptionInfo(ctx context.Context, teamID string) (*models.TeamSubscription, error)
}

// OrderRepository defines the interface for order data operations
type OrderRepository interface {
	CreateOrder(ctx context.Context, order *models.Order) error
	FindOrderByID(ctx context.Context, id string) (*models.Order, error)
	FindOrderByLemonSqueezyID(ctx context.Context, lemonSqueezyID string) (*models.Order, error)
	FindOrdersByTeam(ctx context.Context, teamID string) ([]models.Order, error)
	FindOrdersBySubscription(ctx context.Context, subscriptionID string) ([]models.Order, error)
	UpdateOrder(ctx context.Context, order *models.Order) error
	UpdateOrderStatus(ctx context.Context, id string, status enums.OrderStatus) error
}

// WebhookEventRepository defines the interface for webhook event data operations
type WebhookEventRepository interface {
	CreateWebhookEvent(ctx context.Context, event *models.WebhookEvent) error
	FindWebhookEventByID(ctx context.Context, id string) (*models.WebhookEvent, error)
	FindUnprocessedWebhookEvents(ctx context.Context, maxRetries int) ([]models.WebhookEvent, error)
	UpdateWebhookEvent(ctx context.Context, event *models.WebhookEvent) error
	MarkWebhookEventProcessed(ctx context.Context, id string) error
	MarkWebhookEventFailed(ctx context.Context, id string, errMsg string) error
	DeleteOldProcessedWebhookEvents(ctx context.Context, olderThan time.Duration) error
}

// BillingRepository combines all billing-related repository interfaces
type BillingRepository interface {
	SubscriptionRepository
	OrderRepository
	WebhookEventRepository
	WithTransaction(ctx context.Context, fn func(tx BillingRepository) error) error
}
