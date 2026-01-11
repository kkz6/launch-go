package billing

import "time"

// GenerateCheckoutURLRequest represents the request to generate a checkout URL
type GenerateCheckoutURLRequest struct {
	PlanID string `json:"plan_id" validate:"required"`
	Annual bool   `json:"annual"`
}

// GenerateCheckoutURLResponse represents the response with the checkout URL
type GenerateCheckoutURLResponse struct {
	URL string `json:"url"`
}

// CancelSubscriptionRequest represents the request to cancel a subscription
type CancelSubscriptionRequest struct {
	SubscriptionID string `json:"subscription_id" validate:"required"`
}

// ResumeSubscriptionRequest represents the request to resume a subscription
type ResumeSubscriptionRequest struct {
	SubscriptionID string `json:"subscription_id" validate:"required"`
}

// ChangeSubscriptionPlanRequest represents the request to change a subscription plan
type ChangeSubscriptionPlanRequest struct {
	SubscriptionID string `json:"subscription_id" validate:"required"`
	VariantID      string `json:"variant_id" validate:"required"`
}

// SubscriptionResponse represents a subscription in API responses
type SubscriptionResponse struct {
	ID               string  `json:"id"`
	Status           string  `json:"status"`
	Plan             *Plan   `json:"plan,omitempty"`
	Yearly           bool    `json:"yearly"`
	Renewal          *string `json:"renewal,omitempty"`
	TrialEndsAt      *string `json:"trial_ends_at,omitempty"`
	EndsAt           *string `json:"ends_at,omitempty"`
	CardBrand        *string `json:"card_brand,omitempty"`
	CardLastFour     *string `json:"card_last_four,omitempty"`
	PaymentMethodURL string  `json:"payment_method_url,omitempty"`
}

// ToSubscriptionResponse converts a Subscription model to SubscriptionResponse
func ToSubscriptionResponse(s *Subscription, plan *Plan, updatePaymentURL string) SubscriptionResponse {
	resp := SubscriptionResponse{
		ID:               s.ID,
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
		renewal := s.RenewsAt.Format(time.RFC3339)
		resp.Renewal = &renewal
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
func ToOrderResponse(o *Order) OrderResponse {
	orderedAt := ""
	if o.OrderedAt != nil {
		orderedAt = o.OrderedAt.Format("Jan 2, 2006")
	}

	return OrderResponse{
		OrderedAt:   orderedAt,
		OrderNumber: o.OrderNumber,
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
	SubscriptionPlans []Plan                 `json:"subscription_plans"`
	Receipts          []OrderResponse        `json:"receipts"`
}

// PlanResponse represents a plan in API responses
type PlanResponse struct {
	ID             string      `json:"id"`
	Name           string      `json:"name"`
	MonthlyPricing int64       `json:"monthly_pricing"`
	YearlyPricing  int64       `json:"yearly_pricing"`
	Features       []string    `json:"features"`
	Recommended    bool        `json:"recommended"`
	Options        PlanOptions `json:"options"`
}

// ToPlanResponse converts a Plan to PlanResponse
func ToPlanResponse(p *Plan) PlanResponse {
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

// WebhookPayload represents the payload from LemonSqueezy webhook
type WebhookPayload struct {
	Meta LemonSqueezyMeta       `json:"meta"`
	Data LemonSqueezyData       `json:"data"`
}

// LemonSqueezyMeta represents the meta information in webhook payload
type LemonSqueezyMeta struct {
	EventName       string            `json:"event_name"`
	CustomData      map[string]string `json:"custom_data,omitempty"`
	TestMode        bool              `json:"test_mode"`
}

// LemonSqueezyData represents the data in webhook payload
type LemonSqueezyData struct {
	Type          string                   `json:"type"`
	ID            string                   `json:"id"`
	Attributes    LemonSqueezyAttributes   `json:"attributes"`
	Relationships map[string]interface{}   `json:"relationships,omitempty"`
}

// LemonSqueezyAttributes represents the attributes in webhook data
type LemonSqueezyAttributes struct {
	// Common fields
	StoreID       int     `json:"store_id"`
	CustomerID    int     `json:"customer_id"`
	OrderID       *int    `json:"order_id,omitempty"`
	ProductID     int     `json:"product_id"`
	VariantID     int     `json:"variant_id"`
	ProductName   string  `json:"product_name"`
	VariantName   string  `json:"variant_name"`
	Status        string  `json:"status"`
	TestMode      bool    `json:"test_mode"`

	// Subscription specific fields
	TrialEndsAt    *string `json:"trial_ends_at,omitempty"`
	RenewsAt       *string `json:"renews_at,omitempty"`
	EndsAt         *string `json:"ends_at,omitempty"`
	PauseMode      *string `json:"pause_mode,omitempty"`
	ResumesAt      *string `json:"resumes_at,omitempty"`
	BillingAnchor  int     `json:"billing_anchor"`
	CardBrand      *string `json:"card_brand,omitempty"`
	CardLastFour   *string `json:"card_last_four,omitempty"`
	UpdatePaymentMethodURL *string `json:"urls,omitempty"`

	// Order specific fields
	OrderNumber    *int    `json:"order_number,omitempty"`
	Identifier     *string `json:"identifier,omitempty"`
	Currency       *string `json:"currency,omitempty"`
	CurrencyRate   *string `json:"currency_rate,omitempty"`
	Subtotal       *int64  `json:"subtotal,omitempty"`
	DiscountTotal  *int64  `json:"discount_total,omitempty"`
	Tax            *int64  `json:"tax,omitempty"`
	Total          *int64  `json:"total,omitempty"`
	TaxName        *string `json:"tax_name,omitempty"`
	ReceiptURL     *string `json:"urls,omitempty"`

	// Timestamps
	CreatedAt     string  `json:"created_at"`
	UpdatedAt     string  `json:"updated_at"`
}

// CheckoutURLs represents the URLs returned from LemonSqueezy checkout
type CheckoutURLs struct {
	UpdatePaymentMethod string `json:"update_payment_method"`
	CustomerPortal      string `json:"customer_portal"`
}

// LemonSqueezyCheckoutRequest represents a request to create a checkout session
type LemonSqueezyCheckoutRequest struct {
	StoreID       int               `json:"store_id"`
	VariantID     int               `json:"variant_id"`
	CustomPrice   *int64            `json:"custom_price,omitempty"`
	ProductOptions map[string]string `json:"product_options,omitempty"`
	CheckoutOptions CheckoutOptions  `json:"checkout_options,omitempty"`
	CheckoutData   CheckoutData      `json:"checkout_data,omitempty"`
	ExpiresAt     *string           `json:"expires_at,omitempty"`
	Preview       bool              `json:"preview,omitempty"`
}

// CheckoutOptions represents checkout configuration options
type CheckoutOptions struct {
	Embed              bool    `json:"embed,omitempty"`
	Media              bool    `json:"media,omitempty"`
	Logo               bool    `json:"logo,omitempty"`
	Desc               bool    `json:"desc,omitempty"`
	Discount           bool    `json:"discount,omitempty"`
	Dark               bool    `json:"dark,omitempty"`
	SubscriptionPreview bool   `json:"subscription_preview,omitempty"`
	ButtonColor        *string `json:"button_color,omitempty"`
}

// CheckoutData represents the data passed to checkout
type CheckoutData struct {
	Email        string            `json:"email,omitempty"`
	Name         string            `json:"name,omitempty"`
	Custom       map[string]string `json:"custom,omitempty"`
	DiscountCode string            `json:"discount_code,omitempty"`
}

// LemonSqueezyCheckoutResponse represents the checkout creation response
type LemonSqueezyCheckoutResponse struct {
	Data struct {
		ID         string `json:"id"`
		Type       string `json:"type"`
		Attributes struct {
			StoreID   int    `json:"store_id"`
			VariantID int    `json:"variant_id"`
			URL       string `json:"url"`
			ExpiresAt string `json:"expires_at"`
		} `json:"attributes"`
	} `json:"data"`
}
