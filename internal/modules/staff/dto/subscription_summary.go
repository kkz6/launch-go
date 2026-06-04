package dto

import (
	"time"

	billingmodels "github.com/kkz6/launch-go/internal/modules/billing/models"
)

// SubscriptionSummary is the explicit allow-list view of a subscription returned
// by the back-office /admin/users/:id/subscriptions endpoint. This endpoint is
// cross-tenant, so this DTO is a defense-in-depth allow-list: only fields
// enumerated here ever serialize. The internal billing identifiers
// (customer id, provider subscription id, product id, variant id) are treated as
// sensitive and are NEVER exposed. TeamName is folded in so the UI can show
// which team each subscription belongs to.
type SubscriptionSummary struct {
	TeamID       string     `json:"team_id"`
	TeamName     string     `json:"team_name"`
	Status       string     `json:"status"`
	Type         string     `json:"type"`
	TrialEndsAt  *time.Time `json:"trial_ends_at,omitempty"`
	RenewsAt     *time.Time `json:"renews_at,omitempty"`
	EndsAt       *time.Time `json:"ends_at,omitempty"`
	CardBrand    *string    `json:"card_brand,omitempty"`
	CardLastFour *string    `json:"card_last_four,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

// NewSubscriptionSummary maps a Subscription model to its safe summary view,
// folding in the owning team's name from the supplied teamID→name lookup.
func NewSubscriptionSummary(s billingmodels.Subscription, teamNames map[string]string) SubscriptionSummary {
	return SubscriptionSummary{
		TeamID:       s.BillableID,
		TeamName:     teamNames[s.BillableID],
		Status:       s.Status.String(),
		Type:         s.Type,
		TrialEndsAt:  s.TrialEndsAt,
		RenewsAt:     s.RenewsAt,
		EndsAt:       s.EndsAt,
		CardBrand:    s.CardBrand,
		CardLastFour: s.CardLastFour,
		CreatedAt:    s.CreatedAt,
	}
}

// NewSubscriptionSummaries maps a slice of Subscription models to summary views.
func NewSubscriptionSummaries(subscriptions []billingmodels.Subscription, teamNames map[string]string) []SubscriptionSummary {
	summaries := make([]SubscriptionSummary, 0, len(subscriptions))
	for _, s := range subscriptions {
		summaries = append(summaries, NewSubscriptionSummary(s, teamNames))
	}

	return summaries
}
