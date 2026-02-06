package dto

import "time"

// DodoWebhookPayload represents the payload from DodoPayments webhook
type DodoWebhookPayload struct {
	BusinessID string        `json:"business_id"`
	Type       string        `json:"type"`      // e.g., "subscription.active", "payment.succeeded"
	Timestamp  string        `json:"timestamp"` // ISO 8601
	Data       DodoEventData `json:"data"`
}

// DodoEventData represents the data in webhook payload
// This is a union type that can contain different event-specific data
type DodoEventData struct {
	// Common fields
	PayloadType string `json:"payload_type"` // Payment, Subscription, Refund, Dispute

	// Subscription fields (when payload_type is Subscription)
	Subscription *DodoSubscription `json:"subscription,omitempty"`

	// Payment fields (when payload_type is Payment)
	Payment *DodoPayment `json:"payment,omitempty"`

	// Refund fields (when payload_type is Refund)
	Refund *DodoRefund `json:"refund,omitempty"`

	// Dispute fields (when payload_type is Dispute)
	Dispute *DodoDispute `json:"dispute,omitempty"`
}

// DodoSubscription represents subscription data in webhook
type DodoSubscription struct {
	SubscriptionID    string                  `json:"subscription_id"`
	CustomerID        string                  `json:"customer_id"`
	ProductID         string                  `json:"product_id"`
	Status            string                  `json:"status"` // active, cancelled, expired, failed, on_hold, renewed, updated
	PreviousStatus    *string                 `json:"previous_status,omitempty"`
	Currency          string                  `json:"currency"`
	RecurringAmount   int64                   `json:"recurring_amount"`
	CurrentPeriodEnd  *time.Time              `json:"current_period_end,omitempty"`
	TrialPeriodEnd    *time.Time              `json:"trial_period_end,omitempty"`
	CancelledAt       *time.Time              `json:"cancelled_at,omitempty"`
	Metadata          map[string]string       `json:"metadata,omitempty"`
	PaymentMethod     *DodoPaymentMethod      `json:"payment_method,omitempty"`
	SubscriptionMeter []DodoSubscriptionMeter `json:"subscription_meter,omitempty"`
	CreatedAt         time.Time               `json:"created_at"`
	UpdatedAt         time.Time               `json:"updated_at"`
}

// DodoPayment represents payment data in webhook
type DodoPayment struct {
	PaymentID      string             `json:"payment_id"`
	CustomerID     string             `json:"customer_id"`
	ProductID      *string            `json:"product_id,omitempty"`
	SubscriptionID *string            `json:"subscription_id,omitempty"`
	Status         string             `json:"status"` // succeeded, failed, processing, cancelled
	Currency       string             `json:"currency"`
	TotalAmount    int64              `json:"total_amount"`
	Subtotal       int64              `json:"subtotal"`
	Tax            int64              `json:"tax"`
	DiscountAmount int64              `json:"discount_amount"`
	RefundedAmount int64              `json:"refunded_amount"`
	Metadata       map[string]string  `json:"metadata,omitempty"`
	PaymentMethod  *DodoPaymentMethod `json:"payment_method,omitempty"`
	CreatedAt      time.Time          `json:"created_at"`
	UpdatedAt      time.Time          `json:"updated_at"`
}

// DodoRefund represents refund data in webhook
type DodoRefund struct {
	RefundID  string            `json:"refund_id"`
	PaymentID string            `json:"payment_id"`
	Status    string            `json:"status"` // succeeded, failed
	Amount    int64             `json:"amount"`
	Currency  string            `json:"currency"`
	Reason    *string           `json:"reason,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
}

// DodoDispute represents dispute data in webhook
type DodoDispute struct {
	DisputeID    string    `json:"dispute_id"`
	PaymentID    string    `json:"payment_id"`
	Status       string    `json:"status"` // opened, won, lost
	Amount       int64     `json:"amount"`
	Currency     string    `json:"currency"`
	Reason       *string   `json:"reason,omitempty"`
	DisputeStage *string   `json:"dispute_stage,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// DodoPaymentMethod represents payment method details
type DodoPaymentMethod struct {
	Type         string  `json:"type"` // card, bank_transfer, etc.
	CardBrand    *string `json:"card_brand,omitempty"`
	CardLastFour *string `json:"card_last_four,omitempty"`
}

// DodoSubscriptionMeter represents usage meter data
type DodoSubscriptionMeter struct {
	MeterID      string `json:"meter_id"`
	CurrentUsage int64  `json:"current_usage"`
	PeriodStart  string `json:"period_start"`
	PeriodEnd    string `json:"period_end"`
}

// GetTeamID extracts team_id from metadata
func (p *DodoWebhookPayload) GetTeamID() string {
	switch p.Data.PayloadType {
	case "Subscription":
		if p.Data.Subscription != nil && p.Data.Subscription.Metadata != nil {
			return p.Data.Subscription.Metadata["team_id"]
		}
	case "Payment":
		if p.Data.Payment != nil && p.Data.Payment.Metadata != nil {
			return p.Data.Payment.Metadata["team_id"]
		}
	case "Refund":
		if p.Data.Refund != nil && p.Data.Refund.Metadata != nil {
			return p.Data.Refund.Metadata["team_id"]
		}
	case "Dispute":
		// Disputes don't typically have metadata, need to look up via payment
		return ""
	}

	return ""
}

// CheckoutURLs represents the URLs returned from DodoPayments checkout
type CheckoutURLs struct {
	CustomerPortal string `json:"customer_portal"`
}

// DodoCheckoutRequest represents a request to create a checkout session
type DodoCheckoutRequest struct {
	ProductID   string            `json:"product_id"`
	CustomerID  string            `json:"customer_id,omitempty"`
	SuccessURL  string            `json:"success_url,omitempty"`
	CancelURL   string            `json:"cancel_url,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	PaymentLink bool              `json:"payment_link,omitempty"`
}

// DodoCheckoutResponse represents the checkout creation response
type DodoCheckoutResponse struct {
	SessionID string `json:"session_id"`
	URL       string `json:"url"`
}
