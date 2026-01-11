package billing

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

// String returns the string representation of the status
func (s SubscriptionStatus) String() string {
	return string(s)
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

// AllSubscriptionStatuses returns all valid subscription statuses
func AllSubscriptionStatuses() []SubscriptionStatus {
	return []SubscriptionStatus{
		SubscriptionStatusOnTrial,
		SubscriptionStatusActive,
		SubscriptionStatusPaused,
		SubscriptionStatusPastDue,
		SubscriptionStatusUnpaid,
		SubscriptionStatusCancelled,
		SubscriptionStatusExpired,
	}
}

// PlanInterval represents the billing interval for a plan
type PlanInterval string

const (
	PlanIntervalMonthly PlanInterval = "monthly"
	PlanIntervalYearly  PlanInterval = "yearly"
)

// String returns the string representation of the interval
func (p PlanInterval) String() string {
	return string(p)
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

// AllPlanIntervals returns all valid plan intervals
func AllPlanIntervals() []PlanInterval {
	return []PlanInterval{
		PlanIntervalMonthly,
		PlanIntervalYearly,
	}
}

// OrderStatus represents the status of an order
type OrderStatus string

const (
	OrderStatusPending   OrderStatus = "pending"
	OrderStatusPaid      OrderStatus = "paid"
	OrderStatusFailed    OrderStatus = "failed"
	OrderStatusRefunded  OrderStatus = "refunded"
	OrderStatusDisputed  OrderStatus = "disputed"
)

// String returns the string representation of the order status
func (o OrderStatus) String() string {
	return string(o)
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

// AllOrderStatuses returns all valid order statuses
func AllOrderStatuses() []OrderStatus {
	return []OrderStatus{
		OrderStatusPending,
		OrderStatusPaid,
		OrderStatusFailed,
		OrderStatusRefunded,
		OrderStatusDisputed,
	}
}

// WebhookEventType represents types of webhook events from LemonSqueezy
type WebhookEventType string

const (
	WebhookEventSubscriptionCreated           WebhookEventType = "subscription_created"
	WebhookEventSubscriptionUpdated           WebhookEventType = "subscription_updated"
	WebhookEventSubscriptionCancelled         WebhookEventType = "subscription_cancelled"
	WebhookEventSubscriptionResumed           WebhookEventType = "subscription_resumed"
	WebhookEventSubscriptionExpired           WebhookEventType = "subscription_expired"
	WebhookEventSubscriptionPaused            WebhookEventType = "subscription_paused"
	WebhookEventSubscriptionUnpaused          WebhookEventType = "subscription_unpaused"
	WebhookEventSubscriptionPaymentSuccess    WebhookEventType = "subscription_payment_success"
	WebhookEventSubscriptionPaymentFailed     WebhookEventType = "subscription_payment_failed"
	WebhookEventSubscriptionPaymentRecovered  WebhookEventType = "subscription_payment_recovered"
	WebhookEventOrderCreated                  WebhookEventType = "order_created"
	WebhookEventOrderRefunded                 WebhookEventType = "order_refunded"
)

// String returns the string representation of the webhook event type
func (w WebhookEventType) String() string {
	return string(w)
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

// AllWebhookEventTypes returns all valid webhook event types
func AllWebhookEventTypes() []WebhookEventType {
	return []WebhookEventType{
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
}

// UserRole represents user roles for permission checking
type UserRole string

const (
	UserRoleCustomer UserRole = "customer"
	UserRoleManager  UserRole = "manager"
	UserRoleAdmin    UserRole = "admin"
)

// String returns the string representation of the user role
func (u UserRole) String() string {
	return string(u)
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

// AllUserRoles returns all valid user roles
func AllUserRoles() []UserRole {
	return []UserRole{
		UserRoleCustomer,
		UserRoleManager,
		UserRoleAdmin,
	}
}
