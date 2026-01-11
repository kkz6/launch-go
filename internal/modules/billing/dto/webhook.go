package dto

// WebhookPayload represents the payload from LemonSqueezy webhook
type WebhookPayload struct {
	Meta LemonSqueezyMeta `json:"meta"`
	Data LemonSqueezyData `json:"data"`
}

// LemonSqueezyMeta represents the meta information in webhook payload
type LemonSqueezyMeta struct {
	EventName  string            `json:"event_name"`
	CustomData map[string]string `json:"custom_data,omitempty"`
	TestMode   bool              `json:"test_mode"`
}

// LemonSqueezyData represents the data in webhook payload
type LemonSqueezyData struct {
	Type          string                 `json:"type"`
	ID            string                 `json:"id"`
	Attributes    LemonSqueezyAttributes `json:"attributes"`
	Relationships map[string]interface{} `json:"relationships,omitempty"`
}

// LemonSqueezyAttributes represents the attributes in webhook data
type LemonSqueezyAttributes struct {
	// Common fields
	StoreID     int    `json:"store_id"`
	CustomerID  int    `json:"customer_id"`
	OrderID     *int   `json:"order_id,omitempty"`
	ProductID   int    `json:"product_id"`
	VariantID   int    `json:"variant_id"`
	ProductName string `json:"product_name"`
	VariantName string `json:"variant_name"`
	Status      string `json:"status"`
	TestMode    bool   `json:"test_mode"`

	// Subscription specific fields
	TrialEndsAt            *string `json:"trial_ends_at,omitempty"`
	RenewsAt               *string `json:"renews_at,omitempty"`
	EndsAt                 *string `json:"ends_at,omitempty"`
	PauseMode              *string `json:"pause_mode,omitempty"`
	ResumesAt              *string `json:"resumes_at,omitempty"`
	BillingAnchor          int     `json:"billing_anchor"`
	CardBrand              *string `json:"card_brand,omitempty"`
	CardLastFour           *string `json:"card_last_four,omitempty"`
	UpdatePaymentMethodURL *string `json:"urls,omitempty"`

	// Order specific fields
	OrderNumber   *int    `json:"order_number,omitempty"`
	Identifier    *string `json:"identifier,omitempty"`
	Currency      *string `json:"currency,omitempty"`
	CurrencyRate  *string `json:"currency_rate,omitempty"`
	Subtotal      *int64  `json:"subtotal,omitempty"`
	DiscountTotal *int64  `json:"discount_total,omitempty"`
	Tax           *int64  `json:"tax,omitempty"`
	Total         *int64  `json:"total,omitempty"`
	TaxName       *string `json:"tax_name,omitempty"`
	ReceiptURL    *string `json:"receipt_url,omitempty"`

	// Timestamps
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// CheckoutURLs represents the URLs returned from LemonSqueezy checkout
type CheckoutURLs struct {
	UpdatePaymentMethod string `json:"update_payment_method"`
	CustomerPortal      string `json:"customer_portal"`
}

// LemonSqueezyCheckoutRequest represents a request to create a checkout session
type LemonSqueezyCheckoutRequest struct {
	StoreID        int               `json:"store_id"`
	VariantID      int               `json:"variant_id"`
	CustomPrice    *int64            `json:"custom_price,omitempty"`
	ProductOptions map[string]string `json:"product_options,omitempty"`
	CheckoutOptions CheckoutOptions  `json:"checkout_options,omitempty"`
	CheckoutData    CheckoutData     `json:"checkout_data,omitempty"`
	ExpiresAt      *string           `json:"expires_at,omitempty"`
	Preview        bool              `json:"preview,omitempty"`
}

// CheckoutOptions represents checkout configuration options
type CheckoutOptions struct {
	Embed               bool    `json:"embed,omitempty"`
	Media               bool    `json:"media,omitempty"`
	Logo                bool    `json:"logo,omitempty"`
	Desc                bool    `json:"desc,omitempty"`
	Discount            bool    `json:"discount,omitempty"`
	Dark                bool    `json:"dark,omitempty"`
	SubscriptionPreview bool    `json:"subscription_preview,omitempty"`
	ButtonColor         *string `json:"button_color,omitempty"`
}

// CheckoutData represents the data passed to checkout
type CheckoutData struct {
	Email        string            `json:"email,omitempty"`
	Name         string            `json:"name,omitempty"`
	Custom       map[string]string `json:"custom,omitempty"`
	DiscountCode string            `json:"discount_code,omitempty"`
}

// LemonSqueezyCheckoutResponse represents the checkout creation response
type LemonSqueezyCheckoutResponse struct {
	Data struct {
		ID         string `json:"id"`
		Type       string `json:"type"`
		Attributes struct {
			StoreID   int    `json:"store_id"`
			VariantID int    `json:"variant_id"`
			URL       string `json:"url"`
			ExpiresAt string `json:"expires_at"`
		} `json:"attributes"`
	} `json:"data"`
}
