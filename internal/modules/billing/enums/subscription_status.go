package enums

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
