package repositories

import (
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/billing/contracts"
)

// Registry provides access to all billing repositories
type Registry struct {
	db               *gorm.DB
	subscriptionRepo *SubscriptionRepository
	orderRepo        *OrderRepository
	webhookEventRepo *WebhookEventRepository
}

// NewRegistry creates a new billing repository registry
func NewRegistry(db *gorm.DB) *Registry {
	return &Registry{
		db:               db,
		subscriptionRepo: NewSubscriptionRepository(db),
		orderRepo:        NewOrderRepository(db),
		webhookEventRepo: NewWebhookEventRepository(db),
	}
}

// Subscription returns the subscription repository
func (r *Registry) Subscription() contracts.SubscriptionRepository {
	return r.subscriptionRepo
}

// Order returns the order repository
func (r *Registry) Order() contracts.OrderRepository {
	return r.orderRepo
}

// WebhookEvent returns the webhook event repository
func (r *Registry) WebhookEvent() contracts.WebhookEventRepository {
	return r.webhookEventRepo
}

// DB returns the underlying database connection
func (r *Registry) DB() *gorm.DB {
	return r.db
}
