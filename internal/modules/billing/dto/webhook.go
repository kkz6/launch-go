package dto

// CheckoutURLs represents the URLs returned from DodoPayments checkout
type CheckoutURLs struct {
	CustomerPortal string `json:"customer_portal"`
}

// DodoCheckoutRequest represents a request to create a checkout session
type DodoCheckoutRequest struct {
	ProductID   string            `json:"product_id"`
	CustomerID  string            `json:"customer_id,omitempty"`
	SuccessURL  string            `json:"success_url,omitempty"`
	CancelURL   string            `json:"cancel_url,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	PaymentLink bool              `json:"payment_link,omitempty"`
}

// DodoCheckoutResponse represents the checkout creation response
type DodoCheckoutResponse struct {
	SessionID string `json:"session_id"`
	URL       string `json:"url"`
}
