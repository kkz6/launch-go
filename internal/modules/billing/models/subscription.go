package models

import (
	"time"

	"github.com/kkz6/launch-go/internal/modules/billing/enums"
)

// Subscription represents a team's subscription to a plan
type Subscription struct {
	ID             uint                     `gorm:"primaryKey" json:"id"`
	BillableType   string                   `gorm:"size:255;not null;index:idx_billable" json:"billable_type"`
	BillableID     string                   `gorm:"size:26;not null;index:idx_billable" json:"billable_id"`
	Type           string                   `gorm:"size:255;not null" json:"type"`
	LemonSqueezyID string                   `gorm:"size:255;uniqueIndex;not null" json:"lemon_squeezy_id"`
	Status         enums.SubscriptionStatus `gorm:"size:50;not null" json:"status"`
	ProductID      string                   `gorm:"size:255;not null" json:"product_id"`
	VariantID      string                   `gorm:"size:255;not null" json:"variant_id"`
	CardBrand      *string                  `gorm:"size:50" json:"card_brand,omitempty"`
	CardLastFour   *string                  `gorm:"size:4" json:"card_last_four,omitempty"`
	PauseMode      *string                  `gorm:"size:50" json:"pause_mode,omitempty"`
	PauseResumesAt *time.Time               `json:"pause_resumes_at,omitempty"`
	TrialEndsAt    *time.Time               `json:"trial_ends_at,omitempty"`
	RenewsAt       *time.Time               `json:"renews_at,omitempty"`
	EndsAt         *time.Time               `json:"ends_at,omitempty"`
	CreatedAt      time.Time                `json:"created_at"`
	UpdatedAt      time.Time                `json:"updated_at"`
}

// BillableTypeTeam is the billable type for teams (matches Laravel's Modules\Auth\Models\Team)
const BillableTypeTeam = "Modules\\Auth\\Models\\Team"

// TeamID returns the team ID (alias for BillableID when BillableType is Team)
func (s *Subscription) TeamID() string {
	return s.BillableID
}

// TableName returns the table name for GORM
func (Subscription) TableName() string {
	return "lemon_squeezy_subscriptions"
}

// IsActive checks if the subscription is active or on trial
func (s *Subscription) IsActive() bool {
	if s.Status == enums.SubscriptionStatusActive {
		return true
	}

	if s.Status == enums.SubscriptionStatusOnTrial && s.TrialEndsAt != nil {
		return s.TrialEndsAt.After(time.Now())
	}

	return false
}

// OnTrial checks if the subscription is currently on trial
func (s *Subscription) OnTrial() bool {
	if s.Status != enums.SubscriptionStatusOnTrial {
		return false
	}

	if s.TrialEndsAt == nil {
		return false
	}

	return s.TrialEndsAt.After(time.Now())
}

// OnGracePeriod checks if the subscription is on grace period (cancelled but not ended)
func (s *Subscription) OnGracePeriod() bool {
	if s.EndsAt == nil {
		return false
	}

	return s.Status == enums.SubscriptionStatusCancelled && s.EndsAt.After(time.Now())
}

// IsCancelled checks if the subscription is cancelled
func (s *Subscription) IsCancelled() bool {
	return s.Status == enums.SubscriptionStatusCancelled
}

// IsPaused checks if the subscription is paused
func (s *Subscription) IsPaused() bool {
	return s.Status == enums.SubscriptionStatusPaused
}

// HasExpired checks if the subscription has expired
func (s *Subscription) HasExpired() bool {
	if s.Status == enums.SubscriptionStatusExpired {
		return true
	}

	if s.EndsAt != nil && s.EndsAt.Before(time.Now()) {
		return true
	}

	return false
}
