// Package types contains all type definitions for the billing module
package types

import (
	"database/sql/driver"

	"github.com/kkz6/launch-go/internal/pkg/enumtypes"
)

// =============================================================================
// PlanInterval
// =============================================================================

// PlanInterval represents the billing interval for a plan
type PlanInterval string

const (
	PlanIntervalMonthly PlanInterval = "monthly"
	PlanIntervalYearly  PlanInterval = "yearly"
)

var allPlanIntervals = []PlanInterval{
	PlanIntervalMonthly,
	PlanIntervalYearly,
}

// AllPlanIntervals returns all valid plan intervals
func AllPlanIntervals() []PlanInterval {
	return allPlanIntervals
}

// String returns the string representation of the interval
func (p PlanInterval) String() string {
	return string(p)
}

// Label returns a human-readable label for the interval
func (p PlanInterval) Label() string {
	switch p {
	case PlanIntervalMonthly:
		return "Monthly"
	case PlanIntervalYearly:
		return "Yearly"
	default:
		return string(p)
	}
}

// IsValid checks if the interval is valid
func (p PlanInterval) IsValid() bool {
	return p == PlanIntervalMonthly || p == PlanIntervalYearly
}

// IsMonthly checks if the interval is monthly
func (p PlanInterval) IsMonthly() bool {
	return p == PlanIntervalMonthly
}

// IsYearly checks if the interval is yearly
func (p PlanInterval) IsYearly() bool {
	return p == PlanIntervalYearly
}

// Value implements driver.Valuer for database storage
func (p PlanInterval) Value() (driver.Value, error) {
	return enumtypes.Value(p)
}

// Scan implements sql.Scanner for database retrieval
func (p *PlanInterval) Scan(value any) error {
	return enumtypes.Scan(p, value)
}

// =============================================================================
// SubscriptionStatus
// =============================================================================

// SubscriptionStatus represents the status of a subscription
type SubscriptionStatus string

const (
	SubscriptionStatusOnTrial   SubscriptionStatus = "on_trial"
	SubscriptionStatusActive    SubscriptionStatus = "active"
	SubscriptionStatusPaused    SubscriptionStatus = "paused"
	SubscriptionStatusPastDue   SubscriptionStatus = "past_due"
	SubscriptionStatusUnpaid    SubscriptionStatus = "unpaid"
	SubscriptionStatusCancelled SubscriptionStatus = "cancelled"
	SubscriptionStatusExpired   SubscriptionStatus = "expired"
)

var allSubscriptionStatuses = []SubscriptionStatus{
	SubscriptionStatusOnTrial,
	SubscriptionStatusActive,
	SubscriptionStatusPaused,
	SubscriptionStatusPastDue,
	SubscriptionStatusUnpaid,
	SubscriptionStatusCancelled,
	SubscriptionStatusExpired,
}

// AllSubscriptionStatuses returns all valid subscription statuses
func AllSubscriptionStatuses() []SubscriptionStatus {
	return allSubscriptionStatuses
}

// String returns the string representation of the status
func (s SubscriptionStatus) String() string {
	return string(s)
}

// Label returns a human-readable label for the status
func (s SubscriptionStatus) Label() string {
	switch s {
	case SubscriptionStatusOnTrial:
		return "On Trial"
	case SubscriptionStatusActive:
		return "Active"
	case SubscriptionStatusPaused:
		return "Paused"
	case SubscriptionStatusPastDue:
		return "Past Due"
	case SubscriptionStatusUnpaid:
		return "Unpaid"
	case SubscriptionStatusCancelled:
		return "Cancelled"
	case SubscriptionStatusExpired:
		return "Expired"
	default:
		return string(s)
	}
}

// IsValid checks if the status is a valid subscription status
func (s SubscriptionStatus) IsValid() bool {
	switch s {
	case SubscriptionStatusOnTrial,
		SubscriptionStatusActive,
		SubscriptionStatusPaused,
		SubscriptionStatusPastDue,
		SubscriptionStatusUnpaid,
		SubscriptionStatusCancelled,
		SubscriptionStatusExpired:
		return true
	default:
		return false
	}
}

// IsActive checks if the subscription is considered active
func (s SubscriptionStatus) IsActive() bool {
	return s == SubscriptionStatusActive || s == SubscriptionStatusOnTrial
}

// IsCancelled checks if the subscription is cancelled
func (s SubscriptionStatus) IsCancelled() bool {
	return s == SubscriptionStatusCancelled
}

// IsPaused checks if the subscription is paused
func (s SubscriptionStatus) IsPaused() bool {
	return s == SubscriptionStatusPaused
}

// IsOnTrial checks if the subscription is on trial
func (s SubscriptionStatus) IsOnTrial() bool {
	return s == SubscriptionStatusOnTrial
}

// Value implements driver.Valuer for database storage
func (s SubscriptionStatus) Value() (driver.Value, error) {
	return enumtypes.Value(s)
}

// Scan implements sql.Scanner for database retrieval
func (s *SubscriptionStatus) Scan(value any) error {
	return enumtypes.Scan(s, value)
}

// =============================================================================
// OrderStatus
// =============================================================================

// OrderStatus represents the status of an order
type OrderStatus string

const (
	OrderStatusPending  OrderStatus = "pending"
	OrderStatusPaid     OrderStatus = "paid"
	OrderStatusFailed   OrderStatus = "failed"
	OrderStatusRefunded OrderStatus = "refunded"
	OrderStatusDisputed OrderStatus = "disputed"
)

var allOrderStatuses = []OrderStatus{
	OrderStatusPending,
	OrderStatusPaid,
	OrderStatusFailed,
	OrderStatusRefunded,
	OrderStatusDisputed,
}

// AllOrderStatuses returns all valid order statuses
func AllOrderStatuses() []OrderStatus {
	return allOrderStatuses
}

// String returns the string representation of the order status
func (o OrderStatus) String() string {
	return string(o)
}

// Label returns a human-readable label for the order status
func (o OrderStatus) Label() string {
	switch o {
	case OrderStatusPending:
		return "Pending"
	case OrderStatusPaid:
		return "Paid"
	case OrderStatusFailed:
		return "Failed"
	case OrderStatusRefunded:
		return "Refunded"
	case OrderStatusDisputed:
		return "Disputed"
	default:
		return string(o)
	}
}

// IsValid checks if the order status is valid
func (o OrderStatus) IsValid() bool {
	switch o {
	case OrderStatusPending, OrderStatusPaid, OrderStatusFailed, OrderStatusRefunded, OrderStatusDisputed:
		return true
	default:
		return false
	}
}

// IsPaid checks if the order is paid
func (o OrderStatus) IsPaid() bool {
	return o == OrderStatusPaid
}

// Value implements driver.Valuer for database storage
func (o OrderStatus) Value() (driver.Value, error) {
	return enumtypes.Value(o)
}

// Scan implements sql.Scanner for database retrieval
func (o *OrderStatus) Scan(value any) error {
	return enumtypes.Scan(o, value)
}

// =============================================================================
// UserRole
// =============================================================================

// UserRole represents user roles for permission checking
type UserRole string

const (
	UserRoleCustomer UserRole = "customer"
	UserRoleManager  UserRole = "manager"
	UserRoleAdmin    UserRole = "admin"
)

var allUserRoles = []UserRole{
	UserRoleCustomer,
	UserRoleManager,
	UserRoleAdmin,
}

// AllUserRoles returns all valid user roles
func AllUserRoles() []UserRole {
	return allUserRoles
}

// String returns the string representation of the user role
func (u UserRole) String() string {
	return string(u)
}

// Label returns a human-readable label for the user role
func (u UserRole) Label() string {
	switch u {
	case UserRoleCustomer:
		return "Customer"
	case UserRoleManager:
		return "Manager"
	case UserRoleAdmin:
		return "Admin"
	default:
		return string(u)
	}
}

// IsValid checks if the user role is valid
func (u UserRole) IsValid() bool {
	switch u {
	case UserRoleCustomer, UserRoleManager, UserRoleAdmin:
		return true
	default:
		return false
	}
}

// IsAdmin checks if the role is admin or manager
func (u UserRole) IsAdmin() bool {
	return u == UserRoleAdmin || u == UserRoleManager
}

// Value implements driver.Valuer for database storage
func (u UserRole) Value() (driver.Value, error) {
	return enumtypes.Value(u)
}

// Scan implements sql.Scanner for database retrieval
func (u *UserRole) Scan(value any) error {
	return enumtypes.Scan(u, value)
}

// =============================================================================
// WebhookEventType
// =============================================================================

// WebhookEventType represents types of webhook events from DodoPayments
type WebhookEventType string

const (
	// Subscription events
	WebhookEventSubscriptionActive    WebhookEventType = "subscription.active"
	WebhookEventSubscriptionCancelled WebhookEventType = "subscription.cancelled"
	WebhookEventSubscriptionExpired   WebhookEventType = "subscription.expired"
	WebhookEventSubscriptionFailed    WebhookEventType = "subscription.failed"
	WebhookEventSubscriptionOnHold    WebhookEventType = "subscription.on_hold"
	WebhookEventSubscriptionRenewed   WebhookEventType = "subscription.renewed"
	WebhookEventSubscriptionUpdated   WebhookEventType = "subscription.updated"

	// Payment events
	WebhookEventPaymentSucceeded  WebhookEventType = "payment.succeeded"
	WebhookEventPaymentFailed     WebhookEventType = "payment.failed"
	WebhookEventPaymentProcessing WebhookEventType = "payment.processing"
	WebhookEventPaymentCancelled  WebhookEventType = "payment.cancelled"

	// Refund events
	WebhookEventRefundSucceeded WebhookEventType = "refund.succeeded"
	WebhookEventRefundFailed    WebhookEventType = "refund.failed"

	// Dispute events
	WebhookEventDisputeOpened WebhookEventType = "dispute.opened"
	WebhookEventDisputeWon    WebhookEventType = "dispute.won"
	WebhookEventDisputeLost   WebhookEventType = "dispute.lost"
)

var allWebhookEventTypes = []WebhookEventType{
	WebhookEventSubscriptionActive,
	WebhookEventSubscriptionCancelled,
	WebhookEventSubscriptionExpired,
	WebhookEventSubscriptionFailed,
	WebhookEventSubscriptionOnHold,
	WebhookEventSubscriptionRenewed,
	WebhookEventSubscriptionUpdated,
	WebhookEventPaymentSucceeded,
	WebhookEventPaymentFailed,
	WebhookEventPaymentProcessing,
	WebhookEventPaymentCancelled,
	WebhookEventRefundSucceeded,
	WebhookEventRefundFailed,
	WebhookEventDisputeOpened,
	WebhookEventDisputeWon,
	WebhookEventDisputeLost,
}

// AllWebhookEventTypes returns all valid webhook event types
func AllWebhookEventTypes() []WebhookEventType {
	return allWebhookEventTypes
}

// String returns the string representation of the webhook event type
func (w WebhookEventType) String() string {
	return string(w)
}

// Label returns a human-readable label for the webhook event type
func (w WebhookEventType) Label() string {
	switch w {
	case WebhookEventSubscriptionActive:
		return "Subscription Active"
	case WebhookEventSubscriptionCancelled:
		return "Subscription Cancelled"
	case WebhookEventSubscriptionExpired:
		return "Subscription Expired"
	case WebhookEventSubscriptionFailed:
		return "Subscription Failed"
	case WebhookEventSubscriptionOnHold:
		return "Subscription On Hold"
	case WebhookEventSubscriptionRenewed:
		return "Subscription Renewed"
	case WebhookEventSubscriptionUpdated:
		return "Subscription Updated"
	case WebhookEventPaymentSucceeded:
		return "Payment Succeeded"
	case WebhookEventPaymentFailed:
		return "Payment Failed"
	case WebhookEventPaymentProcessing:
		return "Payment Processing"
	case WebhookEventPaymentCancelled:
		return "Payment Cancelled"
	case WebhookEventRefundSucceeded:
		return "Refund Succeeded"
	case WebhookEventRefundFailed:
		return "Refund Failed"
	case WebhookEventDisputeOpened:
		return "Dispute Opened"
	case WebhookEventDisputeWon:
		return "Dispute Won"
	case WebhookEventDisputeLost:
		return "Dispute Lost"
	default:
		return string(w)
	}
}

// IsValid checks if the webhook event type is valid
func (w WebhookEventType) IsValid() bool {
	switch w {
	case WebhookEventSubscriptionActive,
		WebhookEventSubscriptionCancelled,
		WebhookEventSubscriptionExpired,
		WebhookEventSubscriptionFailed,
		WebhookEventSubscriptionOnHold,
		WebhookEventSubscriptionRenewed,
		WebhookEventSubscriptionUpdated,
		WebhookEventPaymentSucceeded,
		WebhookEventPaymentFailed,
		WebhookEventPaymentProcessing,
		WebhookEventPaymentCancelled,
		WebhookEventRefundSucceeded,
		WebhookEventRefundFailed,
		WebhookEventDisputeOpened,
		WebhookEventDisputeWon,
		WebhookEventDisputeLost:
		return true
	default:
		return false
	}
}

// IsSubscriptionEvent checks if the event is a subscription-related event
func (w WebhookEventType) IsSubscriptionEvent() bool {
	switch w {
	case WebhookEventSubscriptionActive,
		WebhookEventSubscriptionCancelled,
		WebhookEventSubscriptionExpired,
		WebhookEventSubscriptionFailed,
		WebhookEventSubscriptionOnHold,
		WebhookEventSubscriptionRenewed,
		WebhookEventSubscriptionUpdated:
		return true
	default:
		return false
	}
}

// IsPaymentEvent checks if the event is a payment-related event
func (w WebhookEventType) IsPaymentEvent() bool {
	switch w {
	case WebhookEventPaymentSucceeded,
		WebhookEventPaymentFailed,
		WebhookEventPaymentProcessing,
		WebhookEventPaymentCancelled:
		return true
	default:
		return false
	}
}

// IsRefundEvent checks if the event is a refund-related event
func (w WebhookEventType) IsRefundEvent() bool {
	return w == WebhookEventRefundSucceeded || w == WebhookEventRefundFailed
}

// IsDisputeEvent checks if the event is a dispute-related event
func (w WebhookEventType) IsDisputeEvent() bool {
	return w == WebhookEventDisputeOpened || w == WebhookEventDisputeWon || w == WebhookEventDisputeLost
}

// Value implements driver.Valuer for database storage
func (w WebhookEventType) Value() (driver.Value, error) {
	return enumtypes.Value(w)
}

// Scan implements sql.Scanner for database retrieval
func (w *WebhookEventType) Scan(value any) error {
	return enumtypes.Scan(w, value)
}
