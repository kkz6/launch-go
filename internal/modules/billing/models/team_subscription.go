package models

import (
	"time"

	"github.com/kkz6/launch-go/internal/modules/billing/enums"
)

// TeamSubscription represents the join between team and subscription for queries
type TeamSubscription struct {
	TeamID         string                   `json:"team_id"`
	SubscriptionID string                   `json:"subscription_id"`
	ProductID      string                   `json:"product_id"`
	VariantID      string                   `json:"variant_id"`
	Status         enums.SubscriptionStatus `json:"status"`
	TrialEndsAt    *time.Time               `json:"trial_ends_at,omitempty"`
	EndsAt         *time.Time               `json:"ends_at,omitempty"`
}

// IsSubscribed checks if the team has an active subscription
func (ts *TeamSubscription) IsSubscribed() bool {
	if ts.Status == enums.SubscriptionStatusActive {
		return true
	}

	if ts.Status == enums.SubscriptionStatusOnTrial && ts.TrialEndsAt != nil {
		return ts.TrialEndsAt.After(time.Now())
	}

	if ts.Status == enums.SubscriptionStatusCancelled && ts.EndsAt != nil {
		return ts.EndsAt.After(time.Now())
	}

	return false
}
