package providers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/kkz6/launch-go/internal/pkg/httpclient"
	"github.com/rs/zerolog"
)

// LemonSqueezy API errors
var (
	ErrLemonSqueezyAPIError     = errors.New("lemon squeezy api error")
	ErrLemonSqueezyUnauthorized = errors.New("lemon squeezy unauthorized")
	ErrLemonSqueezyNotFound     = errors.New("lemon squeezy resource not found")
	ErrLemonSqueezyRateLimited  = errors.New("lemon squeezy rate limited")
)

// checkoutResponseAttrs represents checkout attributes in the response
type checkoutResponseAttrs struct {
	URL string `json:"url"`
}

// checkoutResponseData represents the checkout data wrapper
type checkoutResponseData struct {
	Attributes checkoutResponseAttrs `json:"attributes"`
}

// checkoutResponse represents the full checkout API response
type checkoutResponse struct {
	Data checkoutResponseData `json:"data"`
}

// OrderURLs represents URLs associated with an order
type OrderURLs struct {
	Receipt string `json:"receipt"`
}

// LemonSqueezyConfig holds configuration for LemonSqueezy client
type LemonSqueezyConfig struct {
	APIKey  string
	StoreID int
	BaseURL string
	Timeout time.Duration
}

// DefaultLemonSqueezyConfig returns default configuration
func DefaultLemonSqueezyConfig() *LemonSqueezyConfig {
	return &LemonSqueezyConfig{
		BaseURL: "https://api.lemonsqueezy.com/v1",
		Timeout: 30 * time.Second,
	}
}

// LemonSqueezyClient handles communication with LemonSqueezy API
type LemonSqueezyClient struct {
	config     *LemonSqueezyConfig
	httpClient *http.Client
	logger     *zerolog.Logger
}

// NewLemonSqueezyClient creates a new LemonSqueezy client
func NewLemonSqueezyClient(config *LemonSqueezyConfig, logger *zerolog.Logger) *LemonSqueezyClient {
	if config.BaseURL == "" {
		config.BaseURL = "https://api.lemonsqueezy.com/v1"
	}
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	var httpClient *http.Client
	if config.Timeout == httpclient.DefaultTimeout {
		httpClient = httpclient.Default()
	} else {
		httpClient = httpclient.WithCustomTimeout(config.Timeout)
	}

	return &LemonSqueezyClient{
		config:     config,
		httpClient: httpClient,
		logger:     logger,
	}
}

// lsRequest makes a request to the LemonSqueezy API
func (c *LemonSqueezyClient) lsRequest(ctx context.Context, method, path string, body interface{}) ([]byte, error) {
	reqURL := c.config.BaseURL + path

	builder := httpclient.NewRequest(ctx, method, reqURL).
		BearerAuth(c.config.APIKey).
		WithHeader("Accept", "application/vnd.api+json").
		WithHeader("Content-Type", "application/vnd.api+json")

	if body != nil {
		builder.JSONBody(body)
	}

	req, err := builder.Build()
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	r, err := httpclient.HandleResponse(resp)
	if err != nil {
		return nil, err
	}

	statusHandler := httpclient.NewStatusHandler().
		On(http.StatusUnauthorized, ErrLemonSqueezyUnauthorized).
		On(http.StatusNotFound, ErrLemonSqueezyNotFound).
		On(http.StatusTooManyRequests, ErrLemonSqueezyRateLimited)

	if err = statusHandler.Check(r); err != nil {
		// Log all API errors
		c.logger.Error().
			Int("status", r.StatusCode).
			Str("body", r.String()).
			Msg("LemonSqueezy API error")

		// For unmapped errors, wrap with our API error
		if errors.Is(err, ErrLemonSqueezyUnauthorized) ||
			errors.Is(err, ErrLemonSqueezyNotFound) ||
			errors.Is(err, ErrLemonSqueezyRateLimited) {
			return nil, err
		}
		return nil, fmt.Errorf("%w: status %d", ErrLemonSqueezyAPIError, r.StatusCode)
	}

	return r.Body, nil
}

// CreateCheckout creates a new checkout session
func (c *LemonSqueezyClient) CreateCheckout(ctx context.Context, variantID string, productName string, teamID string, redirectURL string) (string, error) {
	variantIDInt, err := strconv.Atoi(variantID)
	if err != nil {
		return "", fmt.Errorf("invalid variant ID: %w", err)
	}

	checkoutOptions := map[string]interface{}{
		"embed":                true,
		"subscription_preview": false,
	}

	// Add redirect URL to checkout options if provided
	if redirectURL != "" {
		checkoutOptions["redirect_url"] = redirectURL
	}

	payload := map[string]interface{}{
		"data": map[string]interface{}{
			"type": "checkouts",
			"attributes": map[string]interface{}{
				"product_options": map[string]interface{}{
					"name": productName,
				},
				"checkout_options": checkoutOptions,
				"checkout_data": map[string]interface{}{
					"custom": map[string]string{
						"team_id": teamID,
					},
				},
			},
			"relationships": map[string]interface{}{
				"store": map[string]interface{}{
					"data": map[string]interface{}{
						"type": "stores",
						"id":   strconv.Itoa(c.config.StoreID),
					},
				},
				"variant": map[string]interface{}{
					"data": map[string]interface{}{
						"type": "variants",
						"id":   strconv.Itoa(variantIDInt),
					},
				},
			},
		},
	}

	respBody, err := c.lsRequest(ctx, http.MethodPost, "/checkouts", payload)
	if err != nil {
		return "", err
	}

	var result checkoutResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", err
	}

	return result.Data.Attributes.URL, nil
}

// GetSubscription retrieves a subscription by ID
func (c *LemonSqueezyClient) GetSubscription(ctx context.Context, subscriptionID string) (*LemonSqueezySubscription, error) {
	respBody, err := c.lsRequest(ctx, http.MethodGet, "/subscriptions/"+subscriptionID, nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Data LemonSqueezySubscription `json:"data"`
	}

	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, err
	}

	return &result.Data, nil
}

// CancelSubscription cancels a subscription
func (c *LemonSqueezyClient) CancelSubscription(ctx context.Context, subscriptionID string) error {
	_, err := c.lsRequest(ctx, http.MethodDelete, "/subscriptions/"+subscriptionID, nil)
	return err
}

// ResumeSubscription resumes a cancelled subscription
func (c *LemonSqueezyClient) ResumeSubscription(ctx context.Context, subscriptionID string) error {
	payload := map[string]interface{}{
		"data": map[string]interface{}{
			"type": "subscriptions",
			"id":   subscriptionID,
			"attributes": map[string]interface{}{
				"cancelled": false,
			},
		},
	}

	_, err := c.lsRequest(ctx, http.MethodPatch, "/subscriptions/"+subscriptionID, payload)
	return err
}

// PauseSubscription pauses a subscription
func (c *LemonSqueezyClient) PauseSubscription(ctx context.Context, subscriptionID string, mode string, resumesAt *time.Time) error {
	attributes := map[string]interface{}{
		"pause": map[string]interface{}{
			"mode": mode,
		},
	}

	if resumesAt != nil {
		attributes["pause"].(map[string]interface{})["resumes_at"] = resumesAt.Format(time.RFC3339)
	}

	payload := map[string]interface{}{
		"data": map[string]interface{}{
			"type":       "subscriptions",
			"id":         subscriptionID,
			"attributes": attributes,
		},
	}

	_, err := c.lsRequest(ctx, http.MethodPatch, "/subscriptions/"+subscriptionID, payload)
	return err
}

// UnpauseSubscription unpauses a subscription
func (c *LemonSqueezyClient) UnpauseSubscription(ctx context.Context, subscriptionID string) error {
	payload := map[string]interface{}{
		"data": map[string]interface{}{
			"type": "subscriptions",
			"id":   subscriptionID,
			"attributes": map[string]interface{}{
				"pause": nil,
			},
		},
	}

	_, err := c.lsRequest(ctx, http.MethodPatch, "/subscriptions/"+subscriptionID, payload)
	return err
}

// UpdateSubscription updates a subscription (e.g., change variant/plan)
func (c *LemonSqueezyClient) UpdateSubscription(ctx context.Context, subscriptionID string, variantID int) error {
	payload := map[string]interface{}{
		"data": map[string]interface{}{
			"type": "subscriptions",
			"id":   subscriptionID,
			"attributes": map[string]interface{}{
				"variant_id": variantID,
			},
		},
	}

	_, err := c.lsRequest(ctx, http.MethodPatch, "/subscriptions/"+subscriptionID, payload)
	return err
}

// GetUpdatePaymentMethodURL gets the URL to update the payment method
func (c *LemonSqueezyClient) GetUpdatePaymentMethodURL(ctx context.Context, subscriptionID string) (string, error) {
	sub, err := c.GetSubscription(ctx, subscriptionID)
	if err != nil {
		return "", err
	}

	if sub.Attributes.URLs != nil {
		return sub.Attributes.URLs.UpdatePaymentMethod, nil
	}

	return "", nil
}

// GetCustomerPortalURL gets the customer portal URL
func (c *LemonSqueezyClient) GetCustomerPortalURL(ctx context.Context, subscriptionID string) (string, error) {
	sub, err := c.GetSubscription(ctx, subscriptionID)
	if err != nil {
		return "", err
	}

	if sub.Attributes.URLs != nil {
		return sub.Attributes.URLs.CustomerPortal, nil
	}

	return "", nil
}

// LemonSqueezySubscription represents a subscription from the API
type LemonSqueezySubscription struct {
	Type       string                        `json:"type"`
	ID         string                        `json:"id"`
	Attributes LemonSqueezySubscriptionAttrs `json:"attributes"`
}

// LemonSqueezySubscriptionAttrs represents subscription attributes
type LemonSqueezySubscriptionAttrs struct {
	StoreID         int                `json:"store_id"`
	CustomerID      int                `json:"customer_id"`
	OrderID         int                `json:"order_id"`
	ProductID       int                `json:"product_id"`
	VariantID       int                `json:"variant_id"`
	ProductName     string             `json:"product_name"`
	VariantName     string             `json:"variant_name"`
	UserName        string             `json:"user_name"`
	UserEmail       string             `json:"user_email"`
	Status          string             `json:"status"`
	StatusFormatted string             `json:"status_formatted"`
	CardBrand       *string            `json:"card_brand"`
	CardLastFour    *string            `json:"card_last_four"`
	Pause           *LemonSqueezyPause `json:"pause"`
	Cancelled       bool               `json:"cancelled"`
	TrialEndsAt     *string            `json:"trial_ends_at"`
	BillingAnchor   int                `json:"billing_anchor"`
	RenewsAt        *string            `json:"renews_at"`
	EndsAt          *string            `json:"ends_at"`
	URLs            *LemonSqueezyURLs  `json:"urls"`
	CreatedAt       string             `json:"created_at"`
	UpdatedAt       string             `json:"updated_at"`
	TestMode        bool               `json:"test_mode"`
}

// LemonSqueezyPause represents pause information
type LemonSqueezyPause struct {
	Mode      string  `json:"mode"`
	ResumesAt *string `json:"resumes_at"`
}

// LemonSqueezyURLs represents subscription URLs
type LemonSqueezyURLs struct {
	UpdatePaymentMethod string `json:"update_payment_method"`
	CustomerPortal      string `json:"customer_portal"`
}

// ParseLemonSqueezyTime parses a LemonSqueezy timestamp
func ParseLemonSqueezyTime(timeStr *string) *time.Time {
	if timeStr == nil || *timeStr == "" {
		return nil
	}

	t, err := time.Parse(time.RFC3339, *timeStr)
	if err != nil {
		return nil
	}

	return &t
}

// LemonSqueezyOrder represents an order from the API
type LemonSqueezyOrder struct {
	Type       string                 `json:"type"`
	ID         string                 `json:"id"`
	Attributes LemonSqueezyOrderAttrs `json:"attributes"`
}

// LemonSqueezyOrderAttrs represents order attributes
type LemonSqueezyOrderAttrs struct {
	StoreID         int       `json:"store_id"`
	CustomerID      int       `json:"customer_id"`
	Identifier      string    `json:"identifier"`
	OrderNumber     int       `json:"order_number"`
	UserName        string    `json:"user_name"`
	UserEmail       string    `json:"user_email"`
	Currency        string    `json:"currency"`
	CurrencyRate    string    `json:"currency_rate"`
	Subtotal        int64     `json:"subtotal"`
	DiscountTotal   int64     `json:"discount_total"`
	Tax             int64     `json:"tax"`
	Total           int64     `json:"total"`
	TaxName         *string   `json:"tax_name"`
	Status          string    `json:"status"`
	StatusFormatted string    `json:"status_formatted"`
	Refunded        bool      `json:"refunded"`
	RefundedAt      *string   `json:"refunded_at"`
	URLs            OrderURLs `json:"urls"`
	CreatedAt       string    `json:"created_at"`
	UpdatedAt       string    `json:"updated_at"`
	TestMode        bool      `json:"test_mode"`
}
