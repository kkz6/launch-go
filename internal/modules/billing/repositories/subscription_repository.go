package repositories

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/billing/enums"
	"github.com/kkz6/launch-go/internal/modules/billing/models"
)

// SubscriptionRepository handles database operations for subscriptions
type SubscriptionRepository struct {
	db *gorm.DB
}

// NewSubscriptionRepository creates a new subscription repository
func NewSubscriptionRepository(db *gorm.DB) *SubscriptionRepository {
	return &SubscriptionRepository{db: db}
}

// Create creates a new subscription
func (r *SubscriptionRepository) Create(ctx context.Context, subscription *models.Subscription) error {
	return r.db.WithContext(ctx).Create(subscription).Error
}

// FindByID finds a subscription by ID
func (r *SubscriptionRepository) FindByID(ctx context.Context, id string) (*models.Subscription, error) {
	var subscription models.Subscription
	err := r.db.WithContext(ctx).First(&subscription, "id = ?", id).Error
	if err != nil {
		return nil, err
	}

	return &subscription, nil
}

// FindByLemonSqueezyID finds a subscription by LemonSqueezy ID
func (r *SubscriptionRepository) FindByLemonSqueezyID(ctx context.Context, lemonSqueezyID string) (*models.Subscription, error) {
	var subscription models.Subscription
	err := r.db.WithContext(ctx).First(&subscription, "lemon_squeezy_id = ?", lemonSqueezyID).Error
	if err != nil {
		return nil, err
	}

	return &subscription, nil
}

// FindByTeam finds all subscriptions for a team
func (r *SubscriptionRepository) FindByTeam(ctx context.Context, teamID string) ([]models.Subscription, error) {
	var subscriptions []models.Subscription
	err := r.db.WithContext(ctx).
		Where("billable_type IN ? AND billable_id = ?", models.TeamBillableTypes(), teamID).
		Order("created_at DESC").
		Find(&subscriptions).Error

	return subscriptions, err
}

// FindActiveByTeam finds the active subscription for a team
func (r *SubscriptionRepository) FindActiveByTeam(ctx context.Context, teamID string) (*models.Subscription, error) {
	var subscription models.Subscription
	err := r.db.WithContext(ctx).
		Where("billable_type IN ? AND billable_id = ?", models.TeamBillableTypes(), teamID).
		Where("status IN ?", []enums.SubscriptionStatus{
			enums.SubscriptionStatusActive,
			enums.SubscriptionStatusOnTrial,
		}).
		First(&subscription).Error

	if err != nil {
		return nil, err
	}

	return &subscription, nil
}

// Update updates a subscription
func (r *SubscriptionRepository) Update(ctx context.Context, subscription *models.Subscription) error {
	return r.db.WithContext(ctx).Save(subscription).Error
}

// UpdateStatus updates only the status of a subscription
func (r *SubscriptionRepository) UpdateStatus(ctx context.Context, id string, status enums.SubscriptionStatus) error {
	return r.db.WithContext(ctx).
		Model(&models.Subscription{}).
		Where("id = ?", id).
		Update("status", status).Error
}

// UpdateFields updates specific fields of a subscription
func (r *SubscriptionRepository) UpdateFields(ctx context.Context, id string, fields map[string]interface{}) error {
	return r.db.WithContext(ctx).
		Model(&models.Subscription{}).
		Where("id = ?", id).
		Updates(fields).Error
}

// Delete soft deletes a subscription
func (r *SubscriptionRepository) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&models.Subscription{}, "id = ?", id).Error
}

// CountActiveByTeam counts active subscriptions for a team
func (r *SubscriptionRepository) CountActiveByTeam(ctx context.Context, teamID string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&models.Subscription{}).
		Where("billable_type IN ? AND billable_id = ?", models.TeamBillableTypes(), teamID).
		Where("status IN ?", []enums.SubscriptionStatus{
			enums.SubscriptionStatusActive,
			enums.SubscriptionStatusOnTrial,
		}).
		Count(&count).Error

	return count, err
}

// IsTeamSubscribed checks if a team has an active subscription
func (r *SubscriptionRepository) IsTeamSubscribed(ctx context.Context, teamID string) (bool, error) {
	count, err := r.CountActiveByTeam(ctx, teamID)
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// GetTeamSubscriptionInfo gets subscription information for a team
func (r *SubscriptionRepository) GetTeamSubscriptionInfo(ctx context.Context, teamID string) (*models.TeamSubscription, error) {
	subscription, err := r.FindActiveByTeam(ctx, teamID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}

	return &models.TeamSubscription{
		TeamID:         subscription.BillableID,
		SubscriptionID: fmt.Sprintf("%d", subscription.ID),
		ProductID:      subscription.ProductID,
		VariantID:      subscription.VariantID,
		Status:         subscription.Status,
		TrialEndsAt:    subscription.TrialEndsAt,
		EndsAt:         subscription.EndsAt,
	}, nil
}
