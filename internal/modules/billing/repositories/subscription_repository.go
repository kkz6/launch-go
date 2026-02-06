package repositories

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/billing/models"
	billingtypes "github.com/kkz6/launch-go/internal/modules/billing/types"
	"github.com/kkz6/launch-go/internal/pkg/repository"
)

// SubscriptionRepository handles database operations for subscriptions
type SubscriptionRepository struct {
	repository.Base[models.Subscription]
}

// NewSubscriptionRepository creates a new subscription repository
func NewSubscriptionRepository(db *gorm.DB) *SubscriptionRepository {
	return &SubscriptionRepository{
		Base: repository.NewBase[models.Subscription](db),
	}
}

// Create creates a new subscription
func (r *SubscriptionRepository) Create(ctx context.Context, subscription *models.Subscription) error {
	return r.DB.WithContext(ctx).Create(subscription).Error
}

// FindByID finds a subscription by ID
func (r *SubscriptionRepository) FindByID(ctx context.Context, id string) (*models.Subscription, error) {
	var subscription models.Subscription
	err := r.DB.WithContext(ctx).First(&subscription, "id = ?", id).Error
	if err != nil {
		return nil, err
	}

	return &subscription, nil
}

// FindByProviderSubscriptionID finds a subscription by provider subscription ID
func (r *SubscriptionRepository) FindByProviderSubscriptionID(ctx context.Context, providerSubscriptionID string) (*models.Subscription, error) {
	var subscription models.Subscription
	err := r.DB.WithContext(ctx).First(&subscription, "provider_subscription_id = ?", providerSubscriptionID).Error
	if err != nil {
		return nil, err
	}

	return &subscription, nil
}

// FindByTeam finds all subscriptions for a team
func (r *SubscriptionRepository) FindByTeam(ctx context.Context, teamID string) ([]models.Subscription, error) {
	var subscriptions []models.Subscription
	err := r.DB.WithContext(ctx).
		Where("billable_type IN ? AND billable_id = ?", models.TeamBillableTypes(), teamID).
		Order("created_at DESC").
		Find(&subscriptions).Error

	return subscriptions, err
}

// FindActiveByTeam finds the active subscription for a team
func (r *SubscriptionRepository) FindActiveByTeam(ctx context.Context, teamID string) (*models.Subscription, error) {
	var subscription models.Subscription
	err := r.DB.WithContext(ctx).
		Where("billable_type IN ? AND billable_id = ?", models.TeamBillableTypes(), teamID).
		Where("status IN ?", []billingtypes.SubscriptionStatus{
			billingtypes.SubscriptionStatusActive,
			billingtypes.SubscriptionStatusOnTrial,
		}).
		First(&subscription).Error

	if err != nil {
		return nil, err
	}

	return &subscription, nil
}

// Update updates a subscription
func (r *SubscriptionRepository) Update(ctx context.Context, subscription *models.Subscription) error {
	return r.DB.WithContext(ctx).Save(subscription).Error
}

// UpdateStatus updates only the status of a subscription
func (r *SubscriptionRepository) UpdateStatus(ctx context.Context, id string, status billingtypes.SubscriptionStatus) error {
	return r.DB.WithContext(ctx).
		Model(&models.Subscription{}).
		Where("id = ?", id).
		Update("status", status).Error
}

// UpdateStatusByProviderSubscriptionID updates only the status of a subscription by provider subscription ID
func (r *SubscriptionRepository) UpdateStatusByProviderSubscriptionID(ctx context.Context, providerSubscriptionID string, status billingtypes.SubscriptionStatus) error {
	return r.DB.WithContext(ctx).
		Model(&models.Subscription{}).
		Where("provider_subscription_id = ?", providerSubscriptionID).
		Update("status", status).Error
}

// UpdateFields updates specific fields of a subscription
func (r *SubscriptionRepository) UpdateFields(ctx context.Context, id string, fields map[string]interface{}) error {
	return r.DB.WithContext(ctx).
		Model(&models.Subscription{}).
		Where("id = ?", id).
		Updates(fields).Error
}

// Delete soft deletes a subscription
func (r *SubscriptionRepository) Delete(ctx context.Context, id string) error {
	return r.DB.WithContext(ctx).Delete(&models.Subscription{}, "id = ?", id).Error
}

// CountActiveByTeam counts active subscriptions for a team
func (r *SubscriptionRepository) CountActiveByTeam(ctx context.Context, teamID string) (int64, error) {
	var count int64
	err := r.DB.WithContext(ctx).
		Model(&models.Subscription{}).
		Where("billable_type IN ? AND billable_id = ?", models.TeamBillableTypes(), teamID).
		Where("status IN ?", []billingtypes.SubscriptionStatus{
			billingtypes.SubscriptionStatusActive,
			billingtypes.SubscriptionStatusOnTrial,
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
