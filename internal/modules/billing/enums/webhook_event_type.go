package enums

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
