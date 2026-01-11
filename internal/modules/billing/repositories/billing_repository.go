package repositories

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/billing/enums"
	"github.com/kkz6/launch-go/internal/modules/billing/models"
)

// BillingRepository combines all billing-related repository operations
type BillingRepository struct {
	db                  *gorm.DB
	subscriptionRepo    *SubscriptionRepository
	orderRepo           *OrderRepository
	webhookEventRepo    *WebhookEventRepository
}

// NewBillingRepository creates a new billing repository
func NewBillingRepository(db *gorm.DB) *BillingRepository {
	return &BillingRepository{
		db:               db,
		subscriptionRepo: NewSubscriptionRepository(db),
		orderRepo:        NewOrderRepository(db),
		webhookEventRepo: NewWebhookEventRepository(db),
	}
}

// Subscription methods

// CreateSubscription creates a new subscription
func (r *BillingRepository) CreateSubscription(ctx context.Context, subscription *models.Subscription) error {
	return r.subscriptionRepo.Create(ctx, subscription)
}

// FindSubscriptionByID finds a subscription by ID
func (r *BillingRepository) FindSubscriptionByID(ctx context.Context, id string) (*models.Subscription, error) {
	return r.subscriptionRepo.FindByID(ctx, id)
}

// FindSubscriptionByLemonSqueezyID finds a subscription by LemonSqueezy ID
func (r *BillingRepository) FindSubscriptionByLemonSqueezyID(ctx context.Context, lemonSqueezyID string) (*models.Subscription, error) {
	return r.subscriptionRepo.FindByLemonSqueezyID(ctx, lemonSqueezyID)
}

// FindSubscriptionsByTeam finds all subscriptions for a team
func (r *BillingRepository) FindSubscriptionsByTeam(ctx context.Context, teamID string) ([]models.Subscription, error) {
	return r.subscriptionRepo.FindByTeam(ctx, teamID)
}

// FindActiveSubscriptionByTeam finds the active subscription for a team
func (r *BillingRepository) FindActiveSubscriptionByTeam(ctx context.Context, teamID string) (*models.Subscription, error) {
	return r.subscriptionRepo.FindActiveByTeam(ctx, teamID)
}

// UpdateSubscription updates a subscription
func (r *BillingRepository) UpdateSubscription(ctx context.Context, subscription *models.Subscription) error {
	return r.subscriptionRepo.Update(ctx, subscription)
}

// UpdateSubscriptionStatus updates only the status of a subscription
func (r *BillingRepository) UpdateSubscriptionStatus(ctx context.Context, id string, status enums.SubscriptionStatus) error {
	return r.subscriptionRepo.UpdateStatus(ctx, id, status)
}

// UpdateSubscriptionFields updates specific fields of a subscription
func (r *BillingRepository) UpdateSubscriptionFields(ctx context.Context, id string, fields map[string]interface{}) error {
	return r.subscriptionRepo.UpdateFields(ctx, id, fields)
}

// DeleteSubscription soft deletes a subscription
func (r *BillingRepository) DeleteSubscription(ctx context.Context, id string) error {
	return r.subscriptionRepo.Delete(ctx, id)
}

// CountActiveSubscriptionsByTeam counts active subscriptions for a team
func (r *BillingRepository) CountActiveSubscriptionsByTeam(ctx context.Context, teamID string) (int64, error) {
	return r.subscriptionRepo.CountActiveByTeam(ctx, teamID)
}

// IsTeamSubscribed checks if a team has an active subscription
func (r *BillingRepository) IsTeamSubscribed(ctx context.Context, teamID string) (bool, error) {
	return r.subscriptionRepo.IsTeamSubscribed(ctx, teamID)
}

// GetTeamSubscriptionInfo gets subscription information for a team
func (r *BillingRepository) GetTeamSubscriptionInfo(ctx context.Context, teamID string) (*models.TeamSubscription, error) {
	return r.subscriptionRepo.GetTeamSubscriptionInfo(ctx, teamID)
}

// Order methods

// CreateOrder creates a new order
func (r *BillingRepository) CreateOrder(ctx context.Context, order *models.Order) error {
	return r.orderRepo.Create(ctx, order)
}

// FindOrderByID finds an order by ID
func (r *BillingRepository) FindOrderByID(ctx context.Context, id string) (*models.Order, error) {
	return r.orderRepo.FindByID(ctx, id)
}

// FindOrderByLemonSqueezyID finds an order by LemonSqueezy ID
func (r *BillingRepository) FindOrderByLemonSqueezyID(ctx context.Context, lemonSqueezyID string) (*models.Order, error) {
	return r.orderRepo.FindByLemonSqueezyID(ctx, lemonSqueezyID)
}

// FindOrdersByTeam finds all orders for a team
func (r *BillingRepository) FindOrdersByTeam(ctx context.Context, teamID string) ([]models.Order, error) {
	return r.orderRepo.FindByTeam(ctx, teamID)
}

// FindOrdersBySubscription finds all orders for a subscription
func (r *BillingRepository) FindOrdersBySubscription(ctx context.Context, subscriptionID string) ([]models.Order, error) {
	return r.orderRepo.FindBySubscription(ctx, subscriptionID)
}

// UpdateOrder updates an order
func (r *BillingRepository) UpdateOrder(ctx context.Context, order *models.Order) error {
	return r.orderRepo.Update(ctx, order)
}

// UpdateOrderStatus updates only the status of an order
func (r *BillingRepository) UpdateOrderStatus(ctx context.Context, id string, status enums.OrderStatus) error {
	return r.orderRepo.UpdateStatus(ctx, id, status)
}

// WebhookEvent methods

// CreateWebhookEvent creates a new webhook event
func (r *BillingRepository) CreateWebhookEvent(ctx context.Context, event *models.WebhookEvent) error {
	return r.webhookEventRepo.Create(ctx, event)
}

// FindWebhookEventByID finds a webhook event by ID
func (r *BillingRepository) FindWebhookEventByID(ctx context.Context, id string) (*models.WebhookEvent, error) {
	return r.webhookEventRepo.FindByID(ctx, id)
}

// FindUnprocessedWebhookEvents finds all unprocessed webhook events
func (r *BillingRepository) FindUnprocessedWebhookEvents(ctx context.Context, maxRetries int) ([]models.WebhookEvent, error) {
	return r.webhookEventRepo.FindUnprocessed(ctx, maxRetries)
}

// UpdateWebhookEvent updates a webhook event
func (r *BillingRepository) UpdateWebhookEvent(ctx context.Context, event *models.WebhookEvent) error {
	return r.webhookEventRepo.Update(ctx, event)
}

// MarkWebhookEventProcessed marks a webhook event as processed
func (r *BillingRepository) MarkWebhookEventProcessed(ctx context.Context, id string) error {
	return r.webhookEventRepo.MarkProcessed(ctx, id)
}

// MarkWebhookEventFailed marks a webhook event as failed
func (r *BillingRepository) MarkWebhookEventFailed(ctx context.Context, id string, errMsg string) error {
	return r.webhookEventRepo.MarkFailed(ctx, id, errMsg)
}

// DeleteOldProcessedWebhookEvents deletes processed webhook events older than the specified duration
func (r *BillingRepository) DeleteOldProcessedWebhookEvents(ctx context.Context, olderThan time.Duration) error {
	return r.webhookEventRepo.DeleteOldProcessed(ctx, olderThan)
}

// Transaction helper

// WithTransaction executes a function within a database transaction
func (r *BillingRepository) WithTransaction(ctx context.Context, fn func(tx *BillingRepository) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txRepo := NewBillingRepository(tx)
		return fn(txRepo)
	})
}

// GetDB returns the underlying database connection
func (r *BillingRepository) GetDB() *gorm.DB {
	return r.db
}
