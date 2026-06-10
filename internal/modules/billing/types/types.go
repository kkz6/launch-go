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

var planIntervalLabels = map[PlanInterval]string{
	PlanIntervalMonthly: "Monthly",
	PlanIntervalYearly:  "Yearly",
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
	return enumtypes.Label(p, planIntervalLabels, string(p))
}

// IsValid checks if the interval is valid
func (p PlanInterval) IsValid() bool {
	return enumtypes.IsValid(p, allPlanIntervals...)
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

var subscriptionStatusLabels = map[SubscriptionStatus]string{
	SubscriptionStatusOnTrial:   "On Trial",
	SubscriptionStatusActive:    "Active",
	SubscriptionStatusPaused:    "Paused",
	SubscriptionStatusPastDue:   "Past Due",
	SubscriptionStatusUnpaid:    "Unpaid",
	SubscriptionStatusCancelled: "Cancelled",
	SubscriptionStatusExpired:   "Expired",
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
	return enumtypes.Label(s, subscriptionStatusLabels, string(s))
}

// IsValid checks if the status is a valid subscription status
func (s SubscriptionStatus) IsValid() bool {
	return enumtypes.IsValid(s, allSubscriptionStatuses...)
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

var orderStatusLabels = map[OrderStatus]string{
	OrderStatusPending:  "Pending",
	OrderStatusPaid:     "Paid",
	OrderStatusFailed:   "Failed",
	OrderStatusRefunded: "Refunded",
	OrderStatusDisputed: "Disputed",
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
	return enumtypes.Label(o, orderStatusLabels, string(o))
}

// IsValid checks if the order status is valid
func (o OrderStatus) IsValid() bool {
	return enumtypes.IsValid(o, allOrderStatuses...)
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

var userRoleLabels = map[UserRole]string{
	UserRoleCustomer: "Customer",
	UserRoleManager:  "Manager",
	UserRoleAdmin:    "Admin",
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
	return enumtypes.Label(u, userRoleLabels, string(u))
}

// IsValid checks if the user role is valid
func (u UserRole) IsValid() bool {
	return enumtypes.IsValid(u, allUserRoles...)
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

// WebhookEventType represents types of webhook events from the billing provider
type WebhookEventType string

const (
	// Subscription events (Polar)
	WebhookEventSubscriptionCreated    WebhookEventType = "subscription.created"
	WebhookEventSubscriptionActive     WebhookEventType = "subscription.active"
	WebhookEventSubscriptionUpdated    WebhookEventType = "subscription.updated"
	WebhookEventSubscriptionCanceled   WebhookEventType = "subscription.canceled"
	WebhookEventSubscriptionUncanceled WebhookEventType = "subscription.uncanceled"
	WebhookEventSubscriptionRevoked    WebhookEventType = "subscription.revoked"
	WebhookEventSubscriptionPastDue    WebhookEventType = "subscription.past_due"
	WebhookEventSubscriptionCycled     WebhookEventType = "subscription.cycled"

	// Order events (Polar)
	WebhookEventOrderCreated  WebhookEventType = "order.created"
	WebhookEventOrderPaid     WebhookEventType = "order.paid"
	WebhookEventOrderRefunded WebhookEventType = "order.refunded"
)

var allWebhookEventTypes = []WebhookEventType{
	WebhookEventSubscriptionCreated,
	WebhookEventSubscriptionActive,
	WebhookEventSubscriptionUpdated,
	WebhookEventSubscriptionCanceled,
	WebhookEventSubscriptionUncanceled,
	WebhookEventSubscriptionRevoked,
	WebhookEventSubscriptionPastDue,
	WebhookEventSubscriptionCycled,
	WebhookEventOrderCreated,
	WebhookEventOrderPaid,
	WebhookEventOrderRefunded,
}

var webhookEventTypeLabels = map[WebhookEventType]string{
	WebhookEventSubscriptionCreated:    "Subscription Created",
	WebhookEventSubscriptionActive:     "Subscription Active",
	WebhookEventSubscriptionUpdated:    "Subscription Updated",
	WebhookEventSubscriptionCanceled:   "Subscription Canceled",
	WebhookEventSubscriptionUncanceled: "Subscription Uncanceled",
	WebhookEventSubscriptionRevoked:    "Subscription Revoked",
	WebhookEventSubscriptionPastDue:    "Subscription Past Due",
	WebhookEventSubscriptionCycled:     "Subscription Renewed",
	WebhookEventOrderCreated:           "Order Created",
	WebhookEventOrderPaid:              "Order Paid",
	WebhookEventOrderRefunded:          "Order Refunded",
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
	return enumtypes.Label(w, webhookEventTypeLabels, string(w))
}

// IsValid checks if the webhook event type is valid
func (w WebhookEventType) IsValid() bool {
	return enumtypes.IsValid(w, allWebhookEventTypes...)
}

// IsSubscriptionEvent checks if the event is a subscription-related event
func (w WebhookEventType) IsSubscriptionEvent() bool {
	switch w {
	case WebhookEventSubscriptionCreated,
		WebhookEventSubscriptionActive,
		WebhookEventSubscriptionUpdated,
		WebhookEventSubscriptionCanceled,
		WebhookEventSubscriptionUncanceled,
		WebhookEventSubscriptionRevoked,
		WebhookEventSubscriptionPastDue,
		WebhookEventSubscriptionCycled:
		return true
	default:
		return false
	}
}

// IsOrderEvent checks if the event is an order-related event
func (w WebhookEventType) IsOrderEvent() bool {
	switch w {
	case WebhookEventOrderCreated,
		WebhookEventOrderPaid,
		WebhookEventOrderRefunded:
		return true
	default:
		return false
	}
}

// Value implements driver.Valuer for database storage
func (w WebhookEventType) Value() (driver.Value, error) {
	return enumtypes.Value(w)
}

// Scan implements sql.Scanner for database retrieval
func (w *WebhookEventType) Scan(value any) error {
	return enumtypes.Scan(w, value)
}
