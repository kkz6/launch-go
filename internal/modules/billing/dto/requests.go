package dto

// GenerateCheckoutURLRequest represents the request to generate a checkout URL
type GenerateCheckoutURLRequest struct {
	Plan string `json:"plan" validate:"required"`
}

// CancelSubscriptionRequest represents the request to cancel a subscription
type CancelSubscriptionRequest struct {
	SubscriptionID string `json:"subscription" validate:"required"`
}

// ResumeSubscriptionRequest represents the request to resume a subscription
type ResumeSubscriptionRequest struct {
	SubscriptionID string `json:"subscription" validate:"required"`
}

// ChangeSubscriptionPlanRequest represents the request to change a subscription plan
type ChangeSubscriptionPlanRequest struct {
	SubscriptionID string `json:"subscription" validate:"required"`
	VariantID      string `json:"variant_id" validate:"required"`
}
