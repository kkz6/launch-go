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

// WebhookEventType represents types of webhook events from LemonSqueezy
type WebhookEventType string

const (
	WebhookEventSubscriptionCreated          WebhookEventType = "subscription_created"
	WebhookEventSubscriptionUpdated          WebhookEventType = "subscription_updated"
	WebhookEventSubscriptionCancelled        WebhookEventType = "subscription_cancelled"
	WebhookEventSubscriptionResumed          WebhookEventType = "subscription_resumed"
	WebhookEventSubscriptionExpired          WebhookEventType = "subscription_expired"
	WebhookEventSubscriptionPaused           WebhookEventType = "subscription_paused"
	WebhookEventSubscriptionUnpaused         WebhookEventType = "subscription_unpaused"
	WebhookEventSubscriptionPaymentSuccess   WebhookEventType = "subscription_payment_success"
	WebhookEventSubscriptionPaymentFailed    WebhookEventType = "subscription_payment_failed"
	WebhookEventSubscriptionPaymentRecovered WebhookEventType = "subscription_payment_recovered"
	WebhookEventOrderCreated                 WebhookEventType = "order_created"
	WebhookEventOrderRefunded                WebhookEventType = "order_refunded"
)

var allWebhookEventTypes = []WebhookEventType{
	WebhookEventSubscriptionCreated,
	WebhookEventSubscriptionUpdated,
	WebhookEventSubscriptionCancelled,
	WebhookEventSubscriptionResumed,
	WebhookEventSubscriptionExpired,
	WebhookEventSubscriptionPaused,
	WebhookEventSubscriptionUnpaused,
	WebhookEventSubscriptionPaymentSuccess,
	WebhookEventSubscriptionPaymentFailed,
	WebhookEventSubscriptionPaymentRecovered,
	WebhookEventOrderCreated,
	WebhookEventOrderRefunded,
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
	case WebhookEventSubscriptionCreated:
		return "Subscription Created"
	case WebhookEventSubscriptionUpdated:
		return "Subscription Updated"
	case WebhookEventSubscriptionCancelled:
		return "Subscription Cancelled"
	case WebhookEventSubscriptionResumed:
		return "Subscription Resumed"
	case WebhookEventSubscriptionExpired:
		return "Subscription Expired"
	case WebhookEventSubscriptionPaused:
		return "Subscription Paused"
	case WebhookEventSubscriptionUnpaused:
		return "Subscription Unpaused"
	case WebhookEventSubscriptionPaymentSuccess:
		return "Subscription Payment Success"
	case WebhookEventSubscriptionPaymentFailed:
		return "Subscription Payment Failed"
	case WebhookEventSubscriptionPaymentRecovered:
		return "Subscription Payment Recovered"
	case WebhookEventOrderCreated:
		return "Order Created"
	case WebhookEventOrderRefunded:
		return "Order Refunded"
	default:
		return string(w)
	}
}

// IsValid checks if the webhook event type is valid
func (w WebhookEventType) IsValid() bool {
	switch w {
	case WebhookEventSubscriptionCreated,
		WebhookEventSubscriptionUpdated,
		WebhookEventSubscriptionCancelled,
		WebhookEventSubscriptionResumed,
		WebhookEventSubscriptionExpired,
		WebhookEventSubscriptionPaused,
		WebhookEventSubscriptionUnpaused,
		WebhookEventSubscriptionPaymentSuccess,
		WebhookEventSubscriptionPaymentFailed,
		WebhookEventSubscriptionPaymentRecovered,
		WebhookEventOrderCreated,
		WebhookEventOrderRefunded:
		return true
	default:
		return false
	}
}

// IsSubscriptionEvent checks if the event is a subscription-related event
func (w WebhookEventType) IsSubscriptionEvent() bool {
	switch w {
	case WebhookEventSubscriptionCreated,
		WebhookEventSubscriptionUpdated,
		WebhookEventSubscriptionCancelled,
		WebhookEventSubscriptionResumed,
		WebhookEventSubscriptionExpired,
		WebhookEventSubscriptionPaused,
		WebhookEventSubscriptionUnpaused,
		WebhookEventSubscriptionPaymentSuccess,
		WebhookEventSubscriptionPaymentFailed,
		WebhookEventSubscriptionPaymentRecovered:
		return true
	default:
		return false
	}
}

// IsOrderEvent checks if the event is an order-related event
func (w WebhookEventType) IsOrderEvent() bool {
	return w == WebhookEventOrderCreated || w == WebhookEventOrderRefunded
}

// Value implements driver.Valuer for database storage
func (w WebhookEventType) Value() (driver.Value, error) {
	return enumtypes.Value(w)
}

// Scan implements sql.Scanner for database retrieval
func (w *WebhookEventType) Scan(value any) error {
	return enumtypes.Scan(w, value)
}
