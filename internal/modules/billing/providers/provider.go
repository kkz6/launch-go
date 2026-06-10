package providers

import "context"

// BillingProvider is the payment-provider contract the billing service and
// webhook handler depend on. Implemented by PolarClient. Keeping the surface
// small and provider-agnostic lets the rest of the module stay decoupled from
// any one SDK's types.
type BillingProvider interface {
	// CreateCheckout starts a hosted checkout for productID and returns the
	// URL to redirect the customer to. teamID is stored in metadata so the
	// webhook can attribute the resulting subscription to the team.
	CreateCheckout(ctx context.Context, productID, productName, teamID, redirectURL, customerEmail, customerName string) (string, error)

	// CancelSubscription schedules the subscription to end at the close of the
	// current billing period (it stays active until then).
	CancelSubscription(ctx context.Context, providerSubscriptionID string) error

	// ResumeSubscription clears a pending period-end cancellation.
	ResumeSubscription(ctx context.Context, providerSubscriptionID string) error

	// UpdateSubscription switches the subscription to a different product/plan.
	UpdateSubscription(ctx context.Context, providerSubscriptionID, productID string) error

	// GetUpdatePaymentMethodURL returns a customer-portal URL where the
	// customer can manage their payment method and subscription.
	GetUpdatePaymentMethodURL(ctx context.Context, customerID string) (string, error)
}
