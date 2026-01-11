package billing

import (
	"time"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/pkg/utils"
)

// Subscription represents a team's subscription to a plan
type Subscription struct {
	ID              string             `gorm:"primaryKey;size:26" json:"id"`
	TeamID          string             `gorm:"size:26;not null;index" json:"team_id"`
	LemonSqueezyID  string             `gorm:"size:255;uniqueIndex;not null" json:"lemon_squeezy_id"`
	OrderID         *string            `gorm:"size:255" json:"order_id,omitempty"`
	ProductID       string             `gorm:"size:255;not null" json:"product_id"`
	VariantID       string             `gorm:"size:255;not null" json:"variant_id"`
	Name            string             `gorm:"size:255;not null" json:"name"`
	Status          SubscriptionStatus `gorm:"size:50;not null;default:'active'" json:"status"`
	CardBrand       *string            `gorm:"size:50" json:"card_brand,omitempty"`
	CardLastFour    *string            `gorm:"size:4" json:"card_last_four,omitempty"`
	TrialEndsAt     *time.Time         `json:"trial_ends_at,omitempty"`
	BillingAnchor   int                `gorm:"default:1" json:"billing_anchor"`
	RenewsAt        *time.Time         `json:"renews_at,omitempty"`
	EndsAt          *time.Time         `json:"ends_at,omitempty"`
	PausedAt        *time.Time         `json:"paused_at,omitempty"`
	ResumesAt       *time.Time         `json:"resumes_at,omitempty"`
	CreatedAt       time.Time          `json:"created_at"`
	UpdatedAt       time.Time          `json:"updated_at"`
	DeletedAt       gorm.DeletedAt     `gorm:"index" json:"-"`
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
	if s.Status == SubscriptionStatusActive {
		return true
	}

	if s.Status == SubscriptionStatusOnTrial && s.TrialEndsAt != nil {
		return s.TrialEndsAt.After(time.Now())
	}

	return false
}

// OnTrial checks if the subscription is currently on trial
func (s *Subscription) OnTrial() bool {
	if s.Status != SubscriptionStatusOnTrial {
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

	return s.Status == SubscriptionStatusCancelled && s.EndsAt.After(time.Now())
}

// IsCancelled checks if the subscription is cancelled
func (s *Subscription) IsCancelled() bool {
	return s.Status == SubscriptionStatusCancelled
}

// IsPaused checks if the subscription is paused
func (s *Subscription) IsPaused() bool {
	return s.Status == SubscriptionStatusPaused
}

// HasExpired checks if the subscription has expired
func (s *Subscription) HasExpired() bool {
	if s.Status == SubscriptionStatusExpired {
		return true
	}

	if s.EndsAt != nil && s.EndsAt.Before(time.Now()) {
		return true
	}

	return false
}

// Order represents a payment order
type Order struct {
	ID              string         `gorm:"primaryKey;size:26" json:"id"`
	TeamID          string         `gorm:"size:26;not null;index" json:"team_id"`
	LemonSqueezyID  string         `gorm:"size:255;uniqueIndex;not null" json:"lemon_squeezy_id"`
	SubscriptionID  *string        `gorm:"size:26;index" json:"subscription_id,omitempty"`
	CustomerID      string         `gorm:"size:255;not null" json:"customer_id"`
	ProductID       string         `gorm:"size:255;not null" json:"product_id"`
	VariantID       string         `gorm:"size:255;not null" json:"variant_id"`
	OrderNumber     string         `gorm:"size:255;not null" json:"order_number"`
	Currency        string         `gorm:"size:3;not null;default:'USD'" json:"currency"`
	CurrencyRate    string         `gorm:"size:50" json:"currency_rate"`
	Subtotal        int64          `gorm:"not null" json:"subtotal"`
	DiscountTotal   int64          `gorm:"default:0" json:"discount_total"`
	Tax             int64          `gorm:"default:0" json:"tax"`
	Total           int64          `gorm:"not null" json:"total"`
	TaxName         *string        `gorm:"size:255" json:"tax_name,omitempty"`
	Status          OrderStatus    `gorm:"size:50;not null;default:'pending'" json:"status"`
	ReceiptURL      *string        `gorm:"size:2048" json:"receipt_url,omitempty"`
	OrderedAt       *time.Time     `json:"ordered_at,omitempty"`
	RefundedAt      *time.Time     `json:"refunded_at,omitempty"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"-"`

	// Relations
	Subscription *Subscription `gorm:"foreignKey:SubscriptionID" json:"subscription,omitempty"`
}

// BeforeCreate hook to generate ULID
func (o *Order) BeforeCreate(tx *gorm.DB) error {
	if o.ID == "" {
		o.ID = utils.NewULID()
	}

	return nil
}

// IsPaid checks if the order is paid
func (o *Order) IsPaid() bool {
	return o.Status == OrderStatusPaid
}

// IsRefunded checks if the order is refunded
func (o *Order) IsRefunded() bool {
	return o.Status == OrderStatusRefunded
}

// FormattedSubtotal returns the formatted subtotal in dollars
func (o *Order) FormattedSubtotal() float64 {
	return float64(o.Subtotal) / 100
}

// FormattedDiscount returns the formatted discount in dollars
func (o *Order) FormattedDiscount() float64 {
	return float64(o.DiscountTotal) / 100
}

// FormattedTax returns the formatted tax in dollars
func (o *Order) FormattedTax() float64 {
	return float64(o.Tax) / 100
}

// FormattedTotal returns the formatted total in dollars
func (o *Order) FormattedTotal() float64 {
	return float64(o.Total) / 100
}

// Plan represents a billing plan configuration (stored in config, not database)
type Plan struct {
	ID             string       `json:"id"`
	Name           string       `json:"name"`
	Description    string       `json:"description,omitempty"`
	MonthlyID      string       `json:"monthly_id"`
	YearlyID       string       `json:"yearly_id"`
	MonthlyPricing int64        `json:"monthly_pricing"`
	YearlyPricing  int64        `json:"yearly_pricing"`
	Features       []string     `json:"features,omitempty"`
	Recommended    bool         `json:"recommended"`
	Options        PlanOptions  `json:"options"`
}

// PlanOptions represents the limits and features for a plan
type PlanOptions struct {
	MaxServers             int  `json:"max_servers"`
	MaxSitesPerServer      int  `json:"max_sites_per_server"`
	MaxDeploymentsPerSite  int  `json:"max_deployments_per_site"`
	MaxTeamMembers         int  `json:"max_team_members"`
	HasBackups             bool `json:"has_backups"`
	HasMonitoring          bool `json:"has_monitoring"`
}

// WebhookEvent represents a webhook event received from LemonSqueezy
type WebhookEvent struct {
	ID            string           `gorm:"primaryKey;size:26" json:"id"`
	EventName     WebhookEventType `gorm:"size:100;not null;index" json:"event_name"`
	Payload       string           `gorm:"type:text;not null" json:"payload"`
	Signature     string           `gorm:"size:255;not null" json:"signature"`
	Processed     bool             `gorm:"default:false" json:"processed"`
	ProcessedAt   *time.Time       `json:"processed_at,omitempty"`
	Error         *string          `gorm:"type:text" json:"error,omitempty"`
	RetryCount    int              `gorm:"default:0" json:"retry_count"`
	CreatedAt     time.Time        `json:"created_at"`
	UpdatedAt     time.Time        `json:"updated_at"`
}

// BeforeCreate hook to generate ULID
func (w *WebhookEvent) BeforeCreate(tx *gorm.DB) error {
	if w.ID == "" {
		w.ID = utils.NewULID()
	}

	return nil
}

// MarkProcessed marks the webhook event as processed
func (w *WebhookEvent) MarkProcessed() {
	w.Processed = true
	now := time.Now()
	w.ProcessedAt = &now
}

// MarkFailed marks the webhook event as failed with an error
func (w *WebhookEvent) MarkFailed(err string) {
	w.Error = &err
	w.RetryCount++
}

// CanRetry checks if the webhook event can be retried
func (w *WebhookEvent) CanRetry(maxRetries int) bool {
	return !w.Processed && w.RetryCount < maxRetries
}

// TeamSubscription represents the join between team and subscription for queries
type TeamSubscription struct {
	TeamID          string             `json:"team_id"`
	SubscriptionID  string             `json:"subscription_id"`
	ProductID       string             `json:"product_id"`
	VariantID       string             `json:"variant_id"`
	Status          SubscriptionStatus `json:"status"`
	TrialEndsAt     *time.Time         `json:"trial_ends_at,omitempty"`
	EndsAt          *time.Time         `json:"ends_at,omitempty"`
}

// IsSubscribed checks if the team has an active subscription
func (ts *TeamSubscription) IsSubscribed() bool {
	if ts.Status == SubscriptionStatusActive {
		return true
	}

	if ts.Status == SubscriptionStatusOnTrial && ts.TrialEndsAt != nil {
		return ts.TrialEndsAt.After(time.Now())
	}

	if ts.Status == SubscriptionStatusCancelled && ts.EndsAt != nil {
		return ts.EndsAt.After(time.Now())
	}

	return false
}
