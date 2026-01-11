package billing

import (
	"context"
	"time"

	"gorm.io/gorm"
)

// Repository handles database operations for billing
type Repository struct {
	db *gorm.DB
}

// NewRepository creates a new billing repository
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// Subscription methods

// CreateSubscription creates a new subscription
func (r *Repository) CreateSubscription(ctx context.Context, subscription *Subscription) error {
	return r.db.WithContext(ctx).Create(subscription).Error
}

// FindSubscriptionByID finds a subscription by ID
func (r *Repository) FindSubscriptionByID(ctx context.Context, id string) (*Subscription, error) {
	var subscription Subscription
	err := r.db.WithContext(ctx).First(&subscription, "id = ?", id).Error
	if err != nil {
		return nil, err
	}

	return &subscription, nil
}

// FindSubscriptionByLemonSqueezyID finds a subscription by LemonSqueezy ID
func (r *Repository) FindSubscriptionByLemonSqueezyID(ctx context.Context, lemonSqueezyID string) (*Subscription, error) {
	var subscription Subscription
	err := r.db.WithContext(ctx).First(&subscription, "lemon_squeezy_id = ?", lemonSqueezyID).Error
	if err != nil {
		return nil, err
	}

	return &subscription, nil
}

// FindSubscriptionsByTeam finds all subscriptions for a team
func (r *Repository) FindSubscriptionsByTeam(ctx context.Context, teamID string) ([]Subscription, error) {
	var subscriptions []Subscription
	err := r.db.WithContext(ctx).
		Where("team_id = ?", teamID).
		Order("created_at DESC").
		Find(&subscriptions).Error

	return subscriptions, err
}

// FindActiveSubscriptionByTeam finds the active subscription for a team
func (r *Repository) FindActiveSubscriptionByTeam(ctx context.Context, teamID string) (*Subscription, error) {
	var subscription Subscription
	err := r.db.WithContext(ctx).
		Where("team_id = ?", teamID).
		Where("status IN ?", []SubscriptionStatus{
			SubscriptionStatusActive,
			SubscriptionStatusOnTrial,
		}).
		First(&subscription).Error

	if err != nil {
		return nil, err
	}

	return &subscription, nil
}

// UpdateSubscription updates a subscription
func (r *Repository) UpdateSubscription(ctx context.Context, subscription *Subscription) error {
	return r.db.WithContext(ctx).Save(subscription).Error
}

// UpdateSubscriptionStatus updates only the status of a subscription
func (r *Repository) UpdateSubscriptionStatus(ctx context.Context, id string, status SubscriptionStatus) error {
	return r.db.WithContext(ctx).
		Model(&Subscription{}).
		Where("id = ?", id).
		Update("status", status).Error
}

// UpdateSubscriptionFields updates specific fields of a subscription
func (r *Repository) UpdateSubscriptionFields(ctx context.Context, id string, fields map[string]interface{}) error {
	return r.db.WithContext(ctx).
		Model(&Subscription{}).
		Where("id = ?", id).
		Updates(fields).Error
}

// DeleteSubscription soft deletes a subscription
func (r *Repository) DeleteSubscription(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&Subscription{}, "id = ?", id).Error
}

// CountActiveSubscriptionsByTeam counts active subscriptions for a team
func (r *Repository) CountActiveSubscriptionsByTeam(ctx context.Context, teamID string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&Subscription{}).
		Where("team_id = ?", teamID).
		Where("status IN ?", []SubscriptionStatus{
			SubscriptionStatusActive,
			SubscriptionStatusOnTrial,
		}).
		Count(&count).Error

	return count, err
}

// Order methods

// CreateOrder creates a new order
func (r *Repository) CreateOrder(ctx context.Context, order *Order) error {
	return r.db.WithContext(ctx).Create(order).Error
}

// FindOrderByID finds an order by ID
func (r *Repository) FindOrderByID(ctx context.Context, id string) (*Order, error) {
	var order Order
	err := r.db.WithContext(ctx).
		Preload("Subscription").
		First(&order, "id = ?", id).Error
	if err != nil {
		return nil, err
	}

	return &order, nil
}

// FindOrderByLemonSqueezyID finds an order by LemonSqueezy ID
func (r *Repository) FindOrderByLemonSqueezyID(ctx context.Context, lemonSqueezyID string) (*Order, error) {
	var order Order
	err := r.db.WithContext(ctx).
		Preload("Subscription").
		First(&order, "lemon_squeezy_id = ?", lemonSqueezyID).Error
	if err != nil {
		return nil, err
	}

	return &order, nil
}

// FindOrdersByTeam finds all orders for a team
func (r *Repository) FindOrdersByTeam(ctx context.Context, teamID string) ([]Order, error) {
	var orders []Order
	err := r.db.WithContext(ctx).
		Preload("Subscription").
		Where("team_id = ?", teamID).
		Order("created_at DESC").
		Find(&orders).Error

	return orders, err
}

// FindOrdersBySubscription finds all orders for a subscription
func (r *Repository) FindOrdersBySubscription(ctx context.Context, subscriptionID string) ([]Order, error) {
	var orders []Order
	err := r.db.WithContext(ctx).
		Where("subscription_id = ?", subscriptionID).
		Order("created_at DESC").
		Find(&orders).Error

	return orders, err
}

// UpdateOrder updates an order
func (r *Repository) UpdateOrder(ctx context.Context, order *Order) error {
	return r.db.WithContext(ctx).Save(order).Error
}

// UpdateOrderStatus updates only the status of an order
func (r *Repository) UpdateOrderStatus(ctx context.Context, id string, status OrderStatus) error {
	return r.db.WithContext(ctx).
		Model(&Order{}).
		Where("id = ?", id).
		Update("status", status).Error
}

// WebhookEvent methods

// CreateWebhookEvent creates a new webhook event
func (r *Repository) CreateWebhookEvent(ctx context.Context, event *WebhookEvent) error {
	return r.db.WithContext(ctx).Create(event).Error
}

// FindWebhookEventByID finds a webhook event by ID
func (r *Repository) FindWebhookEventByID(ctx context.Context, id string) (*WebhookEvent, error) {
	var event WebhookEvent
	err := r.db.WithContext(ctx).First(&event, "id = ?", id).Error
	if err != nil {
		return nil, err
	}

	return &event, nil
}

// FindUnprocessedWebhookEvents finds all unprocessed webhook events
func (r *Repository) FindUnprocessedWebhookEvents(ctx context.Context, maxRetries int) ([]WebhookEvent, error) {
	var events []WebhookEvent
	err := r.db.WithContext(ctx).
		Where("processed = ?", false).
		Where("retry_count < ?", maxRetries).
		Order("created_at ASC").
		Find(&events).Error

	return events, err
}

// UpdateWebhookEvent updates a webhook event
func (r *Repository) UpdateWebhookEvent(ctx context.Context, event *WebhookEvent) error {
	return r.db.WithContext(ctx).Save(event).Error
}

// MarkWebhookEventProcessed marks a webhook event as processed
func (r *Repository) MarkWebhookEventProcessed(ctx context.Context, id string) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&WebhookEvent{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"processed":    true,
			"processed_at": now,
		}).Error
}

// MarkWebhookEventFailed marks a webhook event as failed
func (r *Repository) MarkWebhookEventFailed(ctx context.Context, id string, errMsg string) error {
	return r.db.WithContext(ctx).
		Model(&WebhookEvent{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"error":       errMsg,
			"retry_count": gorm.Expr("retry_count + 1"),
		}).Error
}

// DeleteOldProcessedWebhookEvents deletes processed webhook events older than the specified duration
func (r *Repository) DeleteOldProcessedWebhookEvents(ctx context.Context, olderThan time.Duration) error {
	cutoff := time.Now().Add(-olderThan)
	return r.db.WithContext(ctx).
		Where("processed = ?", true).
		Where("processed_at < ?", cutoff).
		Delete(&WebhookEvent{}).Error
}

// Team subscription helper methods

// IsTeamSubscribed checks if a team has an active subscription
func (r *Repository) IsTeamSubscribed(ctx context.Context, teamID string) (bool, error) {
	count, err := r.CountActiveSubscriptionsByTeam(ctx, teamID)
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// GetTeamSubscriptionInfo gets subscription information for a team
func (r *Repository) GetTeamSubscriptionInfo(ctx context.Context, teamID string) (*TeamSubscription, error) {
	subscription, err := r.FindActiveSubscriptionByTeam(ctx, teamID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}

	return &TeamSubscription{
		TeamID:         subscription.TeamID,
		SubscriptionID: subscription.ID,
		ProductID:      subscription.ProductID,
		VariantID:      subscription.VariantID,
		Status:         subscription.Status,
		TrialEndsAt:    subscription.TrialEndsAt,
		EndsAt:         subscription.EndsAt,
	}, nil
}

// Transaction helper

// WithTransaction executes a function within a database transaction
func (r *Repository) WithTransaction(ctx context.Context, fn func(tx *Repository) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txRepo := &Repository{db: tx}
		return fn(txRepo)
	})
}

// GetDB returns the underlying database connection
func (r *Repository) GetDB() *gorm.DB {
	return r.db
}
