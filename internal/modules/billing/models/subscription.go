package models

import (
	"time"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/billing/enums"
	"github.com/kkz6/launch-go/internal/pkg/utils"
)

// Subscription represents a team's subscription to a plan
type Subscription struct {
	ID             string                   `gorm:"primaryKey;size:26" json:"id"`
	TeamID         string                   `gorm:"size:26;not null;index" json:"team_id"`
	LemonSqueezyID string                   `gorm:"size:255;uniqueIndex;not null" json:"lemon_squeezy_id"`
	OrderID        *string                  `gorm:"size:255" json:"order_id,omitempty"`
	ProductID      string                   `gorm:"size:255;not null" json:"product_id"`
	VariantID      string                   `gorm:"size:255;not null" json:"variant_id"`
	Name           string                   `gorm:"size:255;not null" json:"name"`
	Status         enums.SubscriptionStatus `gorm:"size:50;not null;default:'active'" json:"status"`
	CardBrand      *string                  `gorm:"size:50" json:"card_brand,omitempty"`
	CardLastFour   *string                  `gorm:"size:4" json:"card_last_four,omitempty"`
	TrialEndsAt    *time.Time               `json:"trial_ends_at,omitempty"`
	BillingAnchor  int                      `gorm:"default:1" json:"billing_anchor"`
	RenewsAt       *time.Time               `json:"renews_at,omitempty"`
	EndsAt         *time.Time               `json:"ends_at,omitempty"`
	PausedAt       *time.Time               `json:"paused_at,omitempty"`
	ResumesAt      *time.Time               `json:"resumes_at,omitempty"`
	CreatedAt      time.Time                `json:"created_at"`
	UpdatedAt      time.Time                `json:"updated_at"`
	DeletedAt      gorm.DeletedAt           `gorm:"index" json:"-"`
}

// BeforeCreate hook to generate ULID
func (s *Subscription) BeforeCreate(tx *gorm.DB) error {
	if s.ID == "" {
		s.ID = utils.NewULID()
	}

	return nil
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
