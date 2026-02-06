package models

import (
	"time"

	billingtypes "github.com/kkz6/launch-go/internal/modules/billing/types"
)

// Order represents a payment order
type Order struct {
	ID              uint                     `gorm:"primaryKey" json:"id"`
	BillableType    string                   `gorm:"size:255;not null;index:idx_billable" json:"billable_type"`
	BillableID      string                   `gorm:"size:26;not null;index:idx_billable" json:"billable_id"`
	Provider        string                   `gorm:"size:50;not null;default:dodo_payments" json:"provider"`
	ProviderOrderID string                   `gorm:"size:255;uniqueIndex;not null" json:"provider_order_id"`
	CustomerID      string                   `gorm:"size:255;not null" json:"customer_id"`
	Identifier      string                   `gorm:"size:36;uniqueIndex;not null" json:"identifier"`
	ProductID       string                   `gorm:"size:255;not null;index" json:"product_id"`
	VariantID       string                   `gorm:"size:255;not null;index" json:"variant_id"`
	OrderNumber     int                      `gorm:"uniqueIndex;not null" json:"order_number"`
	Currency        string                   `gorm:"size:3;not null" json:"currency"`
	Subtotal        int64                    `gorm:"not null" json:"subtotal"`
	DiscountTotal   int64                    `gorm:"not null" json:"discount_total"`
	Tax             int64                    `gorm:"not null" json:"tax"`
	Total           int64                    `gorm:"not null" json:"total"`
	TaxName         *string                  `gorm:"size:255" json:"tax_name,omitempty"`
	Status          billingtypes.OrderStatus `gorm:"size:50;not null" json:"status"`
	ReceiptURL      *string                  `gorm:"size:2048" json:"receipt_url,omitempty"`
	Refunded        bool                     `gorm:"not null;default:false" json:"refunded"`
	RefundedAt      *time.Time               `json:"refunded_at,omitempty"`
	OrderedAt       time.Time                `json:"ordered_at"`
	CreatedAt       time.Time                `json:"created_at"`
	UpdatedAt       time.Time                `json:"updated_at"`
}

// TeamID returns the team ID (alias for BillableID when BillableType is Team)
func (o *Order) TeamID() string {
	return o.BillableID
}

// TableName returns the table name for GORM
func (Order) TableName() string {
	return "orders"
}

// IsPaid checks if the order is paid
func (o *Order) IsPaid() bool {
	return o.Status == billingtypes.OrderStatusPaid
}

// IsRefunded checks if the order is refunded
func (o *Order) IsRefunded() bool {
	return o.Status == billingtypes.OrderStatusRefunded
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
