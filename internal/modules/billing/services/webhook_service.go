package services

import (
	"context"
	"fmt"
	"time"

	"github.com/kkz6/launch-go/internal/modules/billing/models"
	"github.com/kkz6/launch-go/internal/modules/billing/repositories"
	billingtypes "github.com/kkz6/launch-go/internal/modules/billing/types"
)

// WebhookService handles webhook event processing. It is provider-agnostic:
// the webhook handler parses the provider payload into the internal DTOs below
// and calls these methods, so this layer never imports a payment SDK.
type WebhookService struct {
	repos *repositories.Registry
}

// NewWebhookService creates a new webhook service
func NewWebhookService(repos *repositories.Registry) *WebhookService {
	return &WebhookService{repos: repos}
}

// WebhookSubscription is the provider-agnostic subscription snapshot a webhook
// carries. Status is already mapped to our internal enum by the handler.
type WebhookSubscription struct {
	ProviderSubscriptionID string
	CustomerID             string
	ProductID              string
	Status                 billingtypes.SubscriptionStatus
	RenewsAt               *time.Time
	EndsAt                 *time.Time
}

// WebhookOrder is the provider-agnostic order snapshot a webhook carries.
type WebhookOrder struct {
	ProviderOrderID string
	CustomerID      string
	ProductID       string
	SubscriptionID  string
	Currency        string
	Subtotal        int64
	Tax             int64
	Total           int64
	ReceiptURL      *string
	OrderedAt       time.Time
	CardBrand       string
	CardLastFour    string
}

// CreateWebhookEvent stores a new webhook event
func (s *WebhookService) CreateWebhookEvent(ctx context.Context, event *models.WebhookEvent) error {
	return s.repos.WebhookEvent().Create(ctx, event)
}

// MarkWebhookEventFailed marks a webhook event as failed with an error message
func (s *WebhookService) MarkWebhookEventFailed(ctx context.Context, eventID string, errMsg string) error {
	return s.repos.WebhookEvent().MarkFailed(ctx, eventID, errMsg)
}

// MarkWebhookEventProcessed marks a webhook event as processed
func (s *WebhookService) MarkWebhookEventProcessed(ctx context.Context, eventID string) error {
	return s.repos.WebhookEvent().MarkProcessed(ctx, eventID)
}

// FindUnprocessedWebhookEvents finds unprocessed webhook events
func (s *WebhookService) FindUnprocessedWebhookEvents(ctx context.Context, maxRetries int) ([]models.WebhookEvent, error) {
	return s.repos.WebhookEvent().FindUnprocessed(ctx, maxRetries)
}

// DeleteOldProcessedWebhookEvents deletes old processed webhook events
func (s *WebhookService) DeleteOldProcessedWebhookEvents(ctx context.Context, olderThan time.Duration) error {
	return s.repos.WebhookEvent().DeleteOldProcessed(ctx, olderThan)
}

// CreateOrUpdateSubscription creates a new subscription or updates an existing one
func (s *WebhookService) CreateOrUpdateSubscription(ctx context.Context, teamID string, sub WebhookSubscription) error {
	existing, err := s.repos.Subscription().FindByProviderSubscriptionID(ctx, sub.ProviderSubscriptionID)
	if err == nil && existing != nil {
		return s.applySubscription(ctx, existing, sub)
	}

	subscription := &models.Subscription{
		BillableType:           models.BillableTypeTeam,
		BillableID:             teamID,
		Type:                   "default",
		Provider:               models.ProviderPolar,
		ProviderSubscriptionID: sub.ProviderSubscriptionID,
		CustomerID:             sub.CustomerID,
		ProductID:              sub.ProductID,
		VariantID:              sub.ProductID,
		Status:                 sub.Status,
		RenewsAt:               sub.RenewsAt,
		EndsAt:                 sub.EndsAt,
	}

	return s.repos.Subscription().Create(ctx, subscription)
}

// applySubscription updates an existing subscription row from webhook data
func (s *WebhookService) applySubscription(ctx context.Context, subscription *models.Subscription, sub WebhookSubscription) error {
	subscription.ProductID = sub.ProductID
	subscription.VariantID = sub.ProductID
	if sub.CustomerID != "" {
		subscription.CustomerID = sub.CustomerID
	}
	subscription.Status = sub.Status
	subscription.RenewsAt = sub.RenewsAt
	subscription.EndsAt = sub.EndsAt

	return s.repos.Subscription().Update(ctx, subscription)
}

// UpdateSubscription updates an existing subscription
func (s *WebhookService) UpdateSubscription(ctx context.Context, sub WebhookSubscription) error {
	subscription, err := s.repos.Subscription().FindByProviderSubscriptionID(ctx, sub.ProviderSubscriptionID)
	if err != nil {
		return err
	}

	return s.applySubscription(ctx, subscription, sub)
}

// CancelSubscriptionByWebhook marks a subscription as cancelled from webhook
func (s *WebhookService) CancelSubscriptionByWebhook(ctx context.Context, providerSubscriptionID string, endsAt *time.Time) error {
	subscription, err := s.repos.Subscription().FindByProviderSubscriptionID(ctx, providerSubscriptionID)
	if err != nil {
		return err
	}

	subscription.Status = billingtypes.SubscriptionStatusCancelled
	subscription.EndsAt = endsAt

	return s.repos.Subscription().Update(ctx, subscription)
}

// ExpireSubscription marks a subscription as expired
func (s *WebhookService) ExpireSubscription(ctx context.Context, providerSubscriptionID string) error {
	return s.repos.Subscription().UpdateStatusByProviderSubscriptionID(ctx, providerSubscriptionID, billingtypes.SubscriptionStatusExpired)
}

// HandleSubscriptionFailed handles a failed subscription
func (s *WebhookService) HandleSubscriptionFailed(ctx context.Context, providerSubscriptionID string) error {
	return s.repos.Subscription().UpdateStatusByProviderSubscriptionID(ctx, providerSubscriptionID, billingtypes.SubscriptionStatusPastDue)
}

// HandleSubscriptionRenewed handles a subscription renewal
func (s *WebhookService) HandleSubscriptionRenewed(ctx context.Context, providerSubscriptionID string, nextBillingDate *time.Time) error {
	subscription, err := s.repos.Subscription().FindByProviderSubscriptionID(ctx, providerSubscriptionID)
	if err != nil {
		return err
	}

	subscription.Status = billingtypes.SubscriptionStatusActive
	subscription.RenewsAt = nextBillingDate

	return s.repos.Subscription().Update(ctx, subscription)
}

// HandlePaymentFailed handles a failed payment
func (s *WebhookService) HandlePaymentFailed(ctx context.Context, providerSubscriptionID string) error {
	return s.repos.Subscription().UpdateStatusByProviderSubscriptionID(ctx, providerSubscriptionID, billingtypes.SubscriptionStatusPastDue)
}

// CreateOrder creates a new order from a payment webhook
func (s *WebhookService) CreateOrder(ctx context.Context, teamID string, in WebhookOrder) error {
	productID := in.ProductID
	// Backfill product id AND team id from the associated subscription. Renewal
	// orders often omit team metadata, so without this the order would be saved
	// with an empty BillableID and never surface in billing history.
	if (productID == "" || teamID == "") && in.SubscriptionID != "" {
		if sub, err := s.repos.Subscription().FindByProviderSubscriptionID(ctx, in.SubscriptionID); err == nil {
			if productID == "" {
				productID = sub.ProductID
			}
			if teamID == "" {
				teamID = sub.BillableID
			}
		}
	}

	if teamID == "" {
		return fmt.Errorf("cannot attribute order %s to a team", in.ProviderOrderID)
	}

	// Idempotency: skip if we already recorded this order.
	if existing, err := s.repos.Order().FindByProviderOrderID(ctx, in.ProviderOrderID); err == nil && existing != nil {
		return nil
	}

	order := &models.Order{
		BillableType:    models.BillableTypeTeam,
		BillableID:      teamID,
		Provider:        models.ProviderPolar,
		ProviderOrderID: in.ProviderOrderID,
		CustomerID:      in.CustomerID,
		Identifier:      in.ProviderOrderID,
		ProductID:       productID,
		VariantID:       productID,
		Currency:        in.Currency,
		Subtotal:        in.Subtotal,
		DiscountTotal:   0,
		Tax:             in.Tax,
		Total:           in.Total,
		TaxName:         nil,
		Status:          billingtypes.OrderStatusPaid,
		ReceiptURL:      in.ReceiptURL,
		Refunded:        false,
		OrderedAt:       in.OrderedAt,
	}

	if err := s.repos.Order().Create(ctx, order); err != nil {
		return err
	}

	// Use the auto-increment ID as the order number
	order.OrderNumber = int(order.ID)
	if err := s.repos.Order().Update(ctx, order); err != nil {
		return err
	}

	// Update card details on the associated subscription
	if in.SubscriptionID != "" && in.CardLastFour != "" {
		s.updateSubscriptionCardInfo(ctx, in.SubscriptionID, in.CardBrand, in.CardLastFour)
	}

	return nil
}

// updateSubscriptionCardInfo updates card brand and last four on a subscription
func (s *WebhookService) updateSubscriptionCardInfo(ctx context.Context, providerSubscriptionID, cardNetwork, cardLastFour string) {
	subscription, err := s.repos.Subscription().FindByProviderSubscriptionID(ctx, providerSubscriptionID)
	if err != nil {
		return
	}

	subscription.CardBrand = stringOrNil(cardNetwork)
	subscription.CardLastFour = stringOrNil(cardLastFour)
	_ = s.repos.Subscription().Update(ctx, subscription)
}

// RefundOrder marks an order as refunded by provider order ID
func (s *WebhookService) RefundOrder(ctx context.Context, providerOrderID string) error {
	order, err := s.repos.Order().FindByProviderOrderID(ctx, providerOrderID)
	if err != nil {
		return err
	}

	now := time.Now()
	order.Status = billingtypes.OrderStatusRefunded
	order.Refunded = true
	order.RefundedAt = &now

	return s.repos.Order().Update(ctx, order)
}

// stringOrNil returns a pointer to s if it is not empty, otherwise nil
func stringOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
