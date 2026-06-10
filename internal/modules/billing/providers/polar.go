package providers

import (
	"context"
	"errors"
	"net/http"

	polargo "github.com/polarsource/polar-go"
	"github.com/polarsource/polar-go/models/apierrors"
	"github.com/polarsource/polar-go/models/components"
	"github.com/polarsource/polar-go/models/operations"
	"github.com/rs/zerolog"
	standardwebhooks "github.com/standard-webhooks/standard-webhooks/libraries/go"
)

// Polar API errors.
var (
	ErrPolarAPIError     = errors.New("polar api error")
	ErrPolarUnauthorized = errors.New("polar unauthorized")
	ErrPolarNotFound     = errors.New("polar resource not found")
	ErrPolarRateLimited  = errors.New("polar rate limited")
)

// PolarConfig holds configuration for the Polar client.
type PolarConfig struct {
	AccessToken    string
	WebhookSecret  string
	OrganizationID string
	// Sandbox selects Polar's sandbox environment when true, production otherwise.
	Sandbox bool
}

// PolarClient implements BillingProvider against the Polar.sh API.
type PolarClient struct {
	client   *polargo.Polar
	config   *PolarConfig
	logger   *zerolog.Logger
	webhook  *standardwebhooks.Webhook
	webhkErr error
}

// Compile-time check that PolarClient satisfies the provider contract.
var _ BillingProvider = (*PolarClient)(nil)

// NewPolarClient creates a new Polar client.
func NewPolarClient(config *PolarConfig, logger *zerolog.Logger) *PolarClient {
	server := polargo.ServerProduction
	if config.Sandbox {
		server = polargo.ServerSandbox
	}

	client := polargo.New(
		polargo.WithServer(server),
		polargo.WithSecurity(config.AccessToken),
	)

	c := &PolarClient{
		client: client,
		config: config,
		logger: logger,
	}

	// Build the Standard Webhooks verifier once. Polar signs webhooks with the
	// Standard Webhooks spec (webhook-id / webhook-signature / webhook-timestamp).
	//
	// Polar's secret (e.g. "polar_whs_…") is a PLAIN string, not the base64
	// secret the Standard Webhooks spec assumes. Polar's own SDKs base64-encode
	// it before handing it to the verifier, so the effective HMAC key is the
	// raw secret-string bytes. NewWebhook would base64-decode the secret (and
	// outright error on the "_" in the prefix), rejecting every webhook — so we
	// use NewWebhookRaw with the secret bytes to match how Polar signs.
	if config.WebhookSecret != "" {
		c.webhook, c.webhkErr = standardwebhooks.NewWebhookRaw([]byte(config.WebhookSecret))
	}

	return c
}

// CreateCheckout starts a hosted checkout session and returns its URL.
func (c *PolarClient) CreateCheckout(ctx context.Context, productID, _, teamID, redirectURL, customerEmail, customerName string) (string, error) {
	req := components.CheckoutCreate{
		Products: []string{productID},
		Metadata: map[string]components.CheckoutCreateMetadata{
			"team_id": components.CreateCheckoutCreateMetadataStr(teamID),
		},
	}
	if redirectURL != "" {
		req.SuccessURL = &redirectURL
	}
	if customerEmail != "" {
		req.CustomerEmail = &customerEmail
	}
	if customerName != "" {
		req.CustomerName = &customerName
	}

	res, err := c.client.Checkouts.Create(ctx, req)
	if err != nil {
		c.logger.Error().Err(err).Str("team_id", teamID).Str("product_id", productID).Msg("Failed to create Polar checkout")
		return "", c.mapError(err)
	}
	if res.Checkout == nil {
		return "", ErrPolarAPIError
	}
	return res.Checkout.URL, nil
}

// CancelSubscription schedules cancellation at the end of the current period.
func (c *PolarClient) CancelSubscription(ctx context.Context, providerSubscriptionID string) error {
	_, err := c.client.Subscriptions.Update(ctx, providerSubscriptionID,
		components.CreateSubscriptionUpdateSubscriptionCancel(components.SubscriptionCancel{
			CancelAtPeriodEnd: true,
		}),
	)
	if err != nil {
		c.logger.Error().Err(err).Str("subscription_id", providerSubscriptionID).Msg("Failed to cancel Polar subscription")
		return c.mapError(err)
	}
	return nil
}

// ResumeSubscription clears a pending period-end cancellation.
func (c *PolarClient) ResumeSubscription(ctx context.Context, providerSubscriptionID string) error {
	_, err := c.client.Subscriptions.Update(ctx, providerSubscriptionID,
		components.CreateSubscriptionUpdateSubscriptionCancel(components.SubscriptionCancel{
			CancelAtPeriodEnd: false,
		}),
	)
	if err != nil {
		c.logger.Error().Err(err).Str("subscription_id", providerSubscriptionID).Msg("Failed to resume Polar subscription")
		return c.mapError(err)
	}
	return nil
}

// UpdateSubscription switches the subscription to a different product.
func (c *PolarClient) UpdateSubscription(ctx context.Context, providerSubscriptionID, productID string) error {
	_, err := c.client.Subscriptions.Update(ctx, providerSubscriptionID,
		components.CreateSubscriptionUpdateSubscriptionUpdateProduct(components.SubscriptionUpdateProduct{
			ProductID: productID,
		}),
	)
	if err != nil {
		c.logger.Error().Err(err).Str("subscription_id", providerSubscriptionID).Str("product_id", productID).Msg("Failed to change Polar subscription plan")
		return c.mapError(err)
	}
	return nil
}

// GetUpdatePaymentMethodURL returns a customer-portal URL for the customer.
func (c *PolarClient) GetUpdatePaymentMethodURL(ctx context.Context, customerID string) (string, error) {
	res, err := c.client.CustomerSessions.Create(ctx,
		operations.CreateCustomerSessionsCreateCustomerSessionCreateCustomerSessionCustomerIDCreate(
			components.CustomerSessionCustomerIDCreate{CustomerID: customerID},
		),
	)
	if err != nil {
		c.logger.Warn().Err(err).Str("customer_id", customerID).Msg("Failed to create Polar customer session")
		return "", c.mapError(err)
	}
	if res.CustomerSession == nil {
		return "", ErrPolarAPIError
	}
	return res.CustomerSession.CustomerPortalURL, nil
}

// ValidateWebhook verifies a Polar webhook's Standard Webhooks signature.
// Returns nil when the payload is authentic.
func (c *PolarClient) ValidateWebhook(payload []byte, headers http.Header) error {
	if c.webhkErr != nil {
		return c.webhkErr
	}
	if c.webhook == nil {
		return errors.New("polar webhook secret not configured")
	}
	return c.webhook.Verify(payload, headers)
}

// CreateProduct creates a monthly-recurring product priced at priceCents (USD)
// in the configured organization and returns its product ID. Used by the
// `billing:polar-setup` console command.
func (c *PolarClient) CreateProduct(ctx context.Context, name string, priceCents int64) (string, error) {
	usd := components.PresentmentCurrencyUsd

	// Note: we intentionally do NOT set OrganizationID here. POLAR_ACCESS_TOKEN
	// is an organization access token (polar_oat_…), which is already scoped to
	// one org — Polar rejects an explicit organization_id in that case and
	// infers it from the token.
	req := components.CreateProductCreateProductCreateRecurring(components.ProductCreateRecurring{
		Name:              name,
		RecurringInterval: components.SubscriptionRecurringIntervalMonth,
		Prices: []components.ProductCreateRecurringPrices{
			components.CreateProductCreateRecurringPricesFixed(components.ProductPriceFixedCreate{
				PriceAmount:   priceCents,
				PriceCurrency: &usd,
			}),
		},
	})

	res, err := c.client.Products.Create(ctx, req)
	if err != nil {
		return "", c.mapError(err)
	}
	if res.Product == nil {
		return "", ErrPolarAPIError
	}
	return res.Product.ID, nil
}

// Client exposes the underlying Polar SDK client for advanced operations.
func (c *PolarClient) Client() *polargo.Polar {
	return c.client
}

// OrganizationID returns the configured Polar organization id.
func (c *PolarClient) OrganizationID() string {
	return c.config.OrganizationID
}

// mapError maps Polar SDK errors to our domain errors.
func (c *PolarClient) mapError(err error) error {
	if err == nil {
		return nil
	}

	var apiErr *apierrors.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.StatusCode {
		case http.StatusUnauthorized:
			return ErrPolarUnauthorized
		case http.StatusNotFound:
			return ErrPolarNotFound
		case http.StatusTooManyRequests:
			return ErrPolarRateLimited
		default:
			c.logger.Error().Int("status", apiErr.StatusCode).Msg("Polar API error")
			return ErrPolarAPIError
		}
	}

	return err
}
