package dto

import (
	"fmt"

	"github.com/kkz6/launch-go/internal/modules/billing/models"
	pkgdto "github.com/kkz6/launch-go/internal/pkg/dto"
)

// GenerateCheckoutURLResponse represents the response with the checkout URL
type GenerateCheckoutURLResponse struct {
	URL string `json:"url"`
}

// SubscriptionResponse represents a subscription in API responses
type SubscriptionResponse struct {
	ID               string       `json:"id"`
	Status           string       `json:"status"`
	Plan             *models.Plan `json:"plan,omitempty"`
	Yearly           bool         `json:"yearly"`
	Renewal          *string      `json:"renewal,omitempty"`
	TrialEndsAt      *string      `json:"trial_ends_at,omitempty"`
	EndsAt           *string      `json:"ends_at,omitempty"`
	CardBrand        *string      `json:"card_brand,omitempty"`
	CardLastFour     *string      `json:"card_last_four,omitempty"`
	PaymentMethodURL string       `json:"payment_method_url,omitempty"`
}

// ToSubscriptionResponse converts a Subscription model to SubscriptionResponse
func ToSubscriptionResponse(s *models.Subscription, plan *models.Plan, updatePaymentURL string) SubscriptionResponse {
	resp := SubscriptionResponse{
		ID:               fmt.Sprintf("%d", s.ID),
		Status:           string(s.Status),
		Plan:             plan,
		CardBrand:        s.CardBrand,
		CardLastFour:     s.CardLastFour,
		PaymentMethodURL: updatePaymentURL,
	}

	if plan != nil {
		resp.Yearly = s.VariantID == plan.YearlyID
	}

	if s.RenewsAt != nil {
		resp.Renewal = pkgdto.FormatTime(s.RenewsAt)
	}

	if s.TrialEndsAt != nil {
		trialEnds := s.TrialEndsAt.Format("Jan 2, 2006")
		resp.TrialEndsAt = &trialEnds
	}

	if s.EndsAt != nil {
		endsAt := s.EndsAt.Format("Jan 2, 2006")
		resp.EndsAt = &endsAt
	}

	return resp
}

// OrderResponse represents an order in API responses
type OrderResponse struct {
	OrderedAt   string  `json:"ordered_at"`
	OrderNumber string  `json:"order_number"`
	Discount    float64 `json:"discount"`
	Subtotal    float64 `json:"subtotal"`
	Total       float64 `json:"total"`
	Tax         float64 `json:"tax"`
	ReceiptURL  *string `json:"receipt_url,omitempty"`
}

// ToOrderResponse converts an Order model to OrderResponse
func ToOrderResponse(o *models.Order) OrderResponse {
	return OrderResponse{
		OrderedAt:   o.OrderedAt.Format("Jan 2, 2006"),
		OrderNumber: fmt.Sprintf("%d", o.OrderNumber),
		Discount:    o.FormattedDiscount(),
		Subtotal:    o.FormattedSubtotal(),
		Total:       o.FormattedTotal(),
		Tax:         o.FormattedTax(),
		ReceiptURL:  o.ReceiptURL,
	}
}

// BillingIndexResponse represents the billing page data
type BillingIndexResponse struct {
	ServerCount       int                    `json:"server_count"`
	Subscriptions     []SubscriptionResponse `json:"subscriptions"`
	SubscriptionPlans []models.Plan          `json:"subscription_plans"`
	Receipts          []OrderResponse        `json:"receipts"`
}

// PlanResponse represents a plan in API responses
type PlanResponse struct {
	ID             string             `json:"id"`
	Name           string             `json:"name"`
	MonthlyPricing int64              `json:"monthly_pricing"`
	YearlyPricing  int64              `json:"yearly_pricing"`
	Features       []string           `json:"features"`
	Recommended    bool               `json:"recommended"`
	Options        models.PlanOptions `json:"options"`
}

// ToPlanResponse converts a Plan to PlanResponse
func ToPlanResponse(p *models.Plan) PlanResponse {
	return PlanResponse{
		ID:             p.ID,
		Name:           p.Name,
		MonthlyPricing: p.MonthlyPricing,
		YearlyPricing:  p.YearlyPricing,
		Features:       p.Features,
		Recommended:    p.Recommended,
		Options:        p.Options,
	}
}

// SubscriptionOptionsResponse represents the team's subscription options/limits
type SubscriptionOptionsResponse struct {
	IsSubscribed          bool `json:"is_subscribed"`
	OnTrial               bool `json:"on_trial"`
	MaxServers            int  `json:"max_servers"`
	MaxSitesPerServer     int  `json:"max_sites_per_server"`
	MaxDeploymentsPerSite int  `json:"max_deployments_per_site"`
	MaxTeamMembers        int  `json:"max_team_members"`
	HasBackups            bool `json:"has_backups"`
	HasMonitoring         bool `json:"has_monitoring"`
	CanCreateServer       bool `json:"can_create_server"`
	ServerCount           int  `json:"server_count"`
}
