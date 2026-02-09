package providers

import (
	"context"
	"errors"
	"net/http"

	"github.com/dodopayments/dodopayments-go"
	"github.com/dodopayments/dodopayments-go/option"
	"github.com/rs/zerolog"
)

// DodoPayments API errors
var (
	ErrDodoPaymentsAPIError     = errors.New("dodo payments api error")
	ErrDodoPaymentsUnauthorized = errors.New("dodo payments unauthorized")
	ErrDodoPaymentsNotFound     = errors.New("dodo payments resource not found")
	ErrDodoPaymentsRateLimited  = errors.New("dodo payments rate limited")
)

// DodoPaymentsConfig holds configuration for DodoPayments client
type DodoPaymentsConfig struct {
	APIKey     string
	WebhookKey string
	TestMode   bool
}

// DodoPaymentsClient handles communication with DodoPayments API
type DodoPaymentsClient struct {
	client *dodopayments.Client
	config *DodoPaymentsConfig
	logger *zerolog.Logger
}

// NewDodoPaymentsClient creates a new DodoPayments client
func NewDodoPaymentsClient(config *DodoPaymentsConfig, logger *zerolog.Logger) *DodoPaymentsClient {
	opts := []option.RequestOption{
		option.WithBearerToken(config.APIKey),
	}

	if config.TestMode {
		opts = append(opts, option.WithEnvironmentTestMode())
	} else {
		opts = append(opts, option.WithEnvironmentLiveMode())
	}

	if config.WebhookKey != "" {
		opts = append(opts, option.WithWebhookKey(config.WebhookKey))
	}

	client := dodopayments.NewClient(opts...)

	return &DodoPaymentsClient{
		client: client,
		config: config,
		logger: logger,
	}
}

// CreateCheckout creates a new checkout session
func (c *DodoPaymentsClient) CreateCheckout(ctx context.Context, productID string, productName string, teamID string, redirectURL string, customerEmail string, customerName string) (string, error) {
	params := dodopayments.CheckoutSessionNewParams{
		CheckoutSessionRequest: dodopayments.CheckoutSessionRequestParam{
			ProductCart: dodopayments.F([]dodopayments.CheckoutSessionRequestProductCartParam{
				{
					ProductID: dodopayments.F(productID),
					Quantity:  dodopayments.F(int64(1)),
				},
			}),
			ReturnURL: dodopayments.F(redirectURL),
			Metadata: dodopayments.F(map[string]string{
				"team_id": teamID,
			}),
		},
	}

	// Prefill customer info if available
	if customerEmail != "" {
		customer := dodopayments.NewCustomerParam{
			Email: dodopayments.F(customerEmail),
		}
		if customerName != "" {
			customer.Name = dodopayments.F(customerName)
		}
		params.CheckoutSessionRequest.Customer = dodopayments.F[dodopayments.CustomerRequestUnionParam](customer)
	}

	session, err := c.client.CheckoutSessions.New(ctx, params)
	if err != nil {
		c.logger.Error().Err(err).Str("team_id", teamID).Str("product_id", productID).Msg("Failed to create checkout session")
		return "", c.mapError(err)
	}

	return session.CheckoutURL, nil
}

// GetSubscription retrieves a subscription by ID
func (c *DodoPaymentsClient) GetSubscription(ctx context.Context, subscriptionID string) (*dodopayments.Subscription, error) {
	subscription, err := c.client.Subscriptions.Get(ctx, subscriptionID)
	if err != nil {
		return nil, c.mapError(err)
	}

	return subscription, nil
}

// CancelSubscription cancels a subscription at the next billing date
func (c *DodoPaymentsClient) CancelSubscription(ctx context.Context, subscriptionID string) error {
	_, err := c.client.Subscriptions.Update(ctx, subscriptionID, dodopayments.SubscriptionUpdateParams{
		CancelAtNextBillingDate: dodopayments.F(true),
	})

	if err != nil {
		c.logger.Error().Err(err).Str("subscription_id", subscriptionID).Msg("Failed to cancel subscription")
		return c.mapError(err)
	}

	return nil
}

// ResumeSubscription resumes a cancelled subscription by clearing the cancel flag
func (c *DodoPaymentsClient) ResumeSubscription(ctx context.Context, subscriptionID string) error {
	_, err := c.client.Subscriptions.Update(ctx, subscriptionID, dodopayments.SubscriptionUpdateParams{
		CancelAtNextBillingDate: dodopayments.F(false),
	})

	if err != nil {
		c.logger.Error().Err(err).Str("subscription_id", subscriptionID).Msg("Failed to resume subscription")
		return c.mapError(err)
	}

	return nil
}

// PauseSubscription pauses a subscription by setting status to on_hold
func (c *DodoPaymentsClient) PauseSubscription(ctx context.Context, subscriptionID string) error {
	_, err := c.client.Subscriptions.Update(ctx, subscriptionID, dodopayments.SubscriptionUpdateParams{
		Status: dodopayments.F(dodopayments.SubscriptionStatusOnHold),
	})

	if err != nil {
		c.logger.Error().Err(err).Str("subscription_id", subscriptionID).Msg("Failed to pause subscription")
		return c.mapError(err)
	}

	return nil
}

// UnpauseSubscription unpauses a subscription by setting status to active
func (c *DodoPaymentsClient) UnpauseSubscription(ctx context.Context, subscriptionID string) error {
	_, err := c.client.Subscriptions.Update(ctx, subscriptionID, dodopayments.SubscriptionUpdateParams{
		Status: dodopayments.F(dodopayments.SubscriptionStatusActive),
	})

	if err != nil {
		c.logger.Error().Err(err).Str("subscription_id", subscriptionID).Msg("Failed to unpause subscription")
		return c.mapError(err)
	}

	return nil
}

// UpdateSubscription updates a subscription (e.g., change product/plan)
func (c *DodoPaymentsClient) UpdateSubscription(ctx context.Context, subscriptionID string, productID string) error {
	err := c.client.Subscriptions.ChangePlan(ctx, subscriptionID, dodopayments.SubscriptionChangePlanParams{
		ProductID:            dodopayments.F(productID),
		Quantity:             dodopayments.F(int64(1)),
		ProrationBillingMode: dodopayments.F(dodopayments.SubscriptionChangePlanParamsProrationBillingModeProratedImmediately),
	})

	if err != nil {
		c.logger.Error().Err(err).Str("subscription_id", subscriptionID).Str("product_id", productID).Msg("Failed to update subscription")
		return c.mapError(err)
	}

	return nil
}

// GetCustomerPortalURL gets the customer portal URL for a customer
func (c *DodoPaymentsClient) GetCustomerPortalURL(ctx context.Context, customerID string) (string, error) {
	portalSession, err := c.client.Customers.CustomerPortal.New(ctx, customerID, dodopayments.CustomerCustomerPortalNewParams{})
	if err != nil {
		c.logger.Warn().Err(err).Str("customer_id", customerID).Msg("Failed to get customer portal URL")
		return "", c.mapError(err)
	}

	return portalSession.Link, nil
}

// GetUpdatePaymentMethodURL gets the URL to update the payment method
// DodoPayments uses the customer portal for payment method updates
func (c *DodoPaymentsClient) GetUpdatePaymentMethodURL(ctx context.Context, customerID string) (string, error) {
	return c.GetCustomerPortalURL(ctx, customerID)
}

// UnwrapWebhook verifies and parses a webhook payload
func (c *DodoPaymentsClient) UnwrapWebhook(payload []byte, webhookID, signature, timestamp string) (*dodopayments.UnwrapWebhookEvent, error) {
	headers := http.Header{}
	headers.Set("webhook-id", webhookID)
	headers.Set("webhook-signature", signature)
	headers.Set("webhook-timestamp", timestamp)

	return c.client.Webhooks.Unwrap(payload, headers)
}

// mapError maps DodoPayments SDK errors to our domain errors
func (c *DodoPaymentsClient) mapError(err error) error {
	if err == nil {
		return nil
	}

	var apiErr *dodopayments.Error
	if errors.As(err, &apiErr) {
		switch apiErr.StatusCode {
		case 401:
			return ErrDodoPaymentsUnauthorized
		case 404:
			return ErrDodoPaymentsNotFound
		case 429:
			return ErrDodoPaymentsRateLimited
		default:
			c.logger.Error().
				Int("status", apiErr.StatusCode).
				Msg("DodoPayments API error")
			return ErrDodoPaymentsAPIError
		}
	}

	return err
}

// UnsafeUnwrapWebhook parses a webhook payload without signature verification.
// Use this only for re-processing stored events that were already verified on receipt.
func (c *DodoPaymentsClient) UnsafeUnwrapWebhook(payload []byte) (*dodopayments.UnsafeUnwrapWebhookEvent, error) {
	return c.client.Webhooks.UnsafeUnwrap(payload)
}

// Client returns the underlying DodoPayments client for advanced operations
func (c *DodoPaymentsClient) Client() *dodopayments.Client {
	return c.client
}
