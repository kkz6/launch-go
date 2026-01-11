package dto

// GenerateCheckoutURLRequest represents the request to generate a checkout URL
type GenerateCheckoutURLRequest struct {
	PlanID string `json:"plan_id" validate:"required"`
	Annual bool   `json:"annual"`
}

// CancelSubscriptionRequest represents the request to cancel a subscription
type CancelSubscriptionRequest struct {
	SubscriptionID string `json:"subscription_id" validate:"required"`
}

// ResumeSubscriptionRequest represents the request to resume a subscription
type ResumeSubscriptionRequest struct {
	SubscriptionID string `json:"subscription_id" validate:"required"`
}

// ChangeSubscriptionPlanRequest represents the request to change a subscription plan
type ChangeSubscriptionPlanRequest struct {
	SubscriptionID string `json:"subscription_id" validate:"required"`
	VariantID      string `json:"variant_id" validate:"required"`
}
