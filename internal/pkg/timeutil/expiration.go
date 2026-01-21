package timeutil

import (
	"time"
)

// Expiration wraps a time.Time value and provides expiration-related utility methods.
type Expiration struct {
	time time.Time
}

// ExpiresAt creates an Expiration from a time.Time value.
func ExpiresAt(t time.Time) Expiration {
	return Expiration{time: t}
}

// ExpiresAtPtr creates an *Expiration from a *time.Time value.
// Returns nil if the input is nil.
func ExpiresAtPtr(t *time.Time) *Expiration {
	if t == nil {
		return nil
	}

	exp := ExpiresAt(*t)

	return &exp
}

// Time returns the underlying time.Time value.
func (e Expiration) Time() time.Time {
	return e.time
}

// IsExpired returns true if the expiration time has passed.
func (e Expiration) IsExpired() bool {
	return time.Now().After(e.time)
}

// IsExpiredWithGrace returns true if the expiration time plus the grace period has passed.
func (e Expiration) IsExpiredWithGrace(grace time.Duration) bool {
	return time.Now().After(e.time.Add(grace))
}

// ExpiresWithin returns true if the expiration time is within the given duration from now.
// Returns false if already expired.
func (e Expiration) ExpiresWithin(d time.Duration) bool {
	now := time.Now()

	if now.After(e.time) {
		return false
	}

	return e.time.Before(now.Add(d))
}

// TimeRemaining returns the duration until expiration.
// Returns a negative duration if already expired.
func (e Expiration) TimeRemaining() time.Duration {
	return time.Until(e.time)
}

// DaysRemaining returns the number of whole days until expiration.
// Returns a negative number if already expired.
func (e Expiration) DaysRemaining() int {
	remaining := e.TimeRemaining()

	return int(remaining.Hours() / 24)
}

// StatusThresholds defines the thresholds for determining expiration status levels.
type StatusThresholds struct {
	Critical time.Duration
	Warning  time.Duration
}

// Expiration status constants.
const (
	StatusExpired  = "expired"
	StatusCritical = "critical"
	StatusWarning  = "warning"
	StatusValid    = "valid"
)

// DefaultThresholds returns the default status thresholds:
// Critical: 7 days, Warning: 30 days.
func DefaultThresholds() StatusThresholds {
	return StatusThresholds{
		Critical: 7 * 24 * time.Hour,
		Warning:  30 * 24 * time.Hour,
	}
}

// Status returns the expiration status based on the given thresholds.
// Returns "expired" if already expired, "critical" if within critical threshold,
// "warning" if within warning threshold, or "valid" otherwise.
func (e Expiration) Status(t StatusThresholds) string {
	if e.IsExpired() {
		return StatusExpired
	}

	if e.ExpiresWithin(t.Critical) {
		return StatusCritical
	}

	if e.ExpiresWithin(t.Warning) {
		return StatusWarning
	}

	return StatusValid
}

// Subscription status constants.
const (
	SubscriptionTrialing     = "trialing"
	SubscriptionActive       = "active"
	SubscriptionExpiringSoon = "expiring_soon"
	SubscriptionGracePeriod  = "grace_period"
	SubscriptionExpired      = "expired"
)

// SubscriptionStatus represents the state of a subscription with optional trial and end dates.
type SubscriptionStatus struct {
	TrialEndsAt *time.Time
	EndsAt      *time.Time
	GracePeriod time.Duration
}

// Status returns the current status of the subscription.
// Returns:
//   - "trialing" if within the trial period
//   - "active" if the subscription is active and not expiring soon
//   - "expiring_soon" if the subscription expires within 7 days
//   - "grace_period" if expired but within the grace period
//   - "expired" if expired and past the grace period
func (s SubscriptionStatus) Status() string {
	now := time.Now()

	if s.TrialEndsAt != nil && now.Before(*s.TrialEndsAt) {
		return SubscriptionTrialing
	}

	if s.EndsAt == nil {
		return SubscriptionActive
	}

	exp := ExpiresAt(*s.EndsAt)

	if exp.IsExpired() {
		if s.GracePeriod > 0 && !exp.IsExpiredWithGrace(s.GracePeriod) {
			return SubscriptionGracePeriod
		}

		return SubscriptionExpired
	}

	if exp.ExpiresWithin(7 * 24 * time.Hour) {
		return SubscriptionExpiringSoon
	}

	return SubscriptionActive
}
