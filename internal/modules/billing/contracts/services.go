package contracts

import (
	"context"

	"github.com/kkz6/launch-go/internal/modules/billing/dto"
	"github.com/kkz6/launch-go/internal/modules/billing/models"
)

// BillingService defines the interface for billing operations
type BillingService interface {
	// Plan operations
	GetPlans() []models.Plan
	GetPlanByID(planID string) *models.Plan
	GetPlanByProductID(productID string) *models.Plan
	GetPlanByVariantID(variantID string) *models.Plan

	// Checkout operations
	GenerateCheckoutURL(ctx context.Context, teamID string, req *dto.GenerateCheckoutURLRequest, redirectURL string, customerEmail string, customerName string) (string, error)

	// Subscription operations
	GetSubscriptions(ctx context.Context, teamID string) ([]models.Subscription, error)
	GetActiveSubscription(ctx context.Context, teamID string) (*models.Subscription, error)
	GetSubscriptionByID(ctx context.Context, id string) (*models.Subscription, error)
	CancelSubscription(ctx context.Context, subscriptionID string) error
	ResumeSubscription(ctx context.Context, subscriptionID string) error

	// Order operations
	GetOrders(ctx context.Context, teamID string) ([]models.Order, error)

	// Subscription status
	IsSubscribed(ctx context.Context, teamID string) (bool, error)

	// Billing data
	GetBillingData(ctx context.Context, teamID string, serverCount int) (*dto.BillingIndexResponse, error)
}

// PaymentProvider defines the interface for payment provider operations
type PaymentProvider interface {
	CreateCheckout(ctx context.Context, variantID string, productName string, teamID string, redirectURL string) (string, error)
	GetSubscription(ctx context.Context, subscriptionID string) (interface{}, error)
	CancelSubscription(ctx context.Context, subscriptionID string) error
	ResumeSubscription(ctx context.Context, subscriptionID string) error
	PauseSubscription(ctx context.Context, subscriptionID string, mode string, resumesAt interface{}) error
	UnpauseSubscription(ctx context.Context, subscriptionID string) error
	UpdateSubscription(ctx context.Context, subscriptionID string, variantID int) error
	GetUpdatePaymentMethodURL(ctx context.Context, subscriptionID string) (string, error)
	GetCustomerPortalURL(ctx context.Context, subscriptionID string) (string, error)
}
