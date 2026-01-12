package models

import (
	"time"

	"github.com/kkz6/launch-go/internal/modules/billing/enums"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

// Order represents a payment order
type Order struct {
	basemodels.BaseModel
	basemodels.SoftDeleteModel
	TeamID         string            `gorm:"size:26;not null;index" json:"team_id"`
	LemonSqueezyID string            `gorm:"size:255;uniqueIndex;not null" json:"lemon_squeezy_id"`
	SubscriptionID *string           `gorm:"size:26;index" json:"subscription_id,omitempty"`
	CustomerID     string            `gorm:"size:255;not null" json:"customer_id"`
	ProductID      string            `gorm:"size:255;not null" json:"product_id"`
	VariantID      string            `gorm:"size:255;not null" json:"variant_id"`
	OrderNumber    string            `gorm:"size:255;not null" json:"order_number"`
	Currency       string            `gorm:"size:3;not null;default:'USD'" json:"currency"`
	CurrencyRate   string            `gorm:"size:50" json:"currency_rate"`
	Subtotal       int64             `gorm:"not null" json:"subtotal"`
	DiscountTotal  int64             `gorm:"default:0" json:"discount_total"`
	Tax            int64             `gorm:"default:0" json:"tax"`
	Total          int64             `gorm:"not null" json:"total"`
	TaxName        *string           `gorm:"size:255" json:"tax_name,omitempty"`
	Status         enums.OrderStatus `gorm:"size:50;not null;default:'pending'" json:"status"`
	ReceiptURL     *string           `gorm:"size:2048" json:"receipt_url,omitempty"`
	OrderedAt      *time.Time        `json:"ordered_at,omitempty"`
	RefundedAt     *time.Time        `json:"refunded_at,omitempty"`

	// Relations
	Subscription *Subscription `gorm:"foreignKey:SubscriptionID" json:"subscription,omitempty"`
}

// IsPaid checks if the order is paid
func (o *Order) IsPaid() bool {
	return o.Status == enums.OrderStatusPaid
}

// IsRefunded checks if the order is refunded
func (o *Order) IsRefunded() bool {
	return o.Status == enums.OrderStatusRefunded
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
