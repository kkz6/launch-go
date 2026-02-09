package services

import (
	"context"
	"time"

	"github.com/dodopayments/dodopayments-go"

	"github.com/kkz6/launch-go/internal/modules/billing/models"
	"github.com/kkz6/launch-go/internal/modules/billing/repositories"
	billingtypes "github.com/kkz6/launch-go/internal/modules/billing/types"
)

// WebhookService handles webhook event processing
type WebhookService struct {
	repos *repositories.Registry
}

// NewWebhookService creates a new webhook service
func NewWebhookService(repos *repositories.Registry) *WebhookService {
	return &WebhookService{repos: repos}
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
func (s *WebhookService) CreateOrUpdateSubscription(ctx context.Context, teamID, providerSubscriptionID string, sub *dodopayments.Subscription) error {
	existing, err := s.repos.Subscription().FindByProviderSubscriptionID(ctx, providerSubscriptionID)
	if err == nil && existing != nil {
		return s.updateSubscriptionFromWebhook(ctx, existing, sub)
	}

	status := MapDodoPaymentsStatus(string(sub.Status))

	nextBillingDate := sub.NextBillingDate
	cancelledAt := sub.CancelledAt

	subscription := &models.Subscription{
		BillableType:           models.BillableTypeTeam,
		BillableID:             teamID,
		Type:                   "default",
		Provider:               models.ProviderDodoPayments,
		ProviderSubscriptionID: providerSubscriptionID,
		CustomerID:             sub.Customer.CustomerID,
		ProductID:              sub.ProductID,
		VariantID:              sub.ProductID, // DodoPayments uses ProductID, no separate variant
		Status:                 status,
		RenewsAt:               &nextBillingDate,
		EndsAt:                 timeOrNil(cancelledAt),
	}

	return s.repos.Subscription().Create(ctx, subscription)
}

// updateSubscriptionFromWebhook updates an existing subscription from webhook data
func (s *WebhookService) updateSubscriptionFromWebhook(ctx context.Context, subscription *models.Subscription, sub *dodopayments.Subscription) error {
	subscription.ProductID = sub.ProductID
	subscription.VariantID = sub.ProductID
	subscription.CustomerID = sub.Customer.CustomerID
	subscription.Status = MapDodoPaymentsStatus(string(sub.Status))

	nextBillingDate := sub.NextBillingDate
	subscription.RenewsAt = &nextBillingDate
	subscription.EndsAt = timeOrNil(sub.CancelledAt)

	return s.repos.Subscription().Update(ctx, subscription)
}

// UpdateSubscription updates an existing subscription
func (s *WebhookService) UpdateSubscription(ctx context.Context, providerSubscriptionID string, sub *dodopayments.Subscription) error {
	subscription, err := s.repos.Subscription().FindByProviderSubscriptionID(ctx, providerSubscriptionID)
	if err != nil {
		return err
	}

	return s.updateSubscriptionFromWebhook(ctx, subscription, sub)
}

// CancelSubscriptionByWebhook marks a subscription as cancelled from webhook
func (s *WebhookService) CancelSubscriptionByWebhook(ctx context.Context, providerSubscriptionID string, cancelledAt time.Time) error {
	subscription, err := s.repos.Subscription().FindByProviderSubscriptionID(ctx, providerSubscriptionID)
	if err != nil {
		return err
	}

	subscription.Status = billingtypes.SubscriptionStatusCancelled
	subscription.EndsAt = timeOrNil(cancelledAt)

	return s.repos.Subscription().Update(ctx, subscription)
}

// ExpireSubscription marks a subscription as expired
func (s *WebhookService) ExpireSubscription(ctx context.Context, providerSubscriptionID string) error {
	return s.repos.Subscription().UpdateStatusByProviderSubscriptionID(ctx, providerSubscriptionID, billingtypes.SubscriptionStatusExpired)
}

// HandleSubscriptionFailed handles a failed subscription
func (s *WebhookService) HandleSubscriptionFailed(ctx context.Context, providerSubscriptionID string) error {
	return s.repos.Subscription().UpdateStatusByProviderSubscriptionID(ctx, providerSubscriptionID, billingtypes.SubscriptionStatusUnpaid)
}

// PauseSubscription pauses a subscription
func (s *WebhookService) PauseSubscription(ctx context.Context, providerSubscriptionID string) error {
	subscription, err := s.repos.Subscription().FindByProviderSubscriptionID(ctx, providerSubscriptionID)
	if err != nil {
		return err
	}

	pauseMode := "on_hold"
	subscription.Status = billingtypes.SubscriptionStatusPaused
	subscription.PauseMode = &pauseMode

	return s.repos.Subscription().Update(ctx, subscription)
}

// UnpauseSubscription unpauses a subscription
func (s *WebhookService) UnpauseSubscription(ctx context.Context, providerSubscriptionID string) error {
	subscription, err := s.repos.Subscription().FindByProviderSubscriptionID(ctx, providerSubscriptionID)
	if err != nil {
		return err
	}

	subscription.Status = billingtypes.SubscriptionStatusActive
	subscription.PauseMode = nil
	subscription.PauseResumesAt = nil

	return s.repos.Subscription().Update(ctx, subscription)
}

// HandleSubscriptionRenewed handles a subscription renewal
func (s *WebhookService) HandleSubscriptionRenewed(ctx context.Context, providerSubscriptionID string, nextBillingDate time.Time) error {
	subscription, err := s.repos.Subscription().FindByProviderSubscriptionID(ctx, providerSubscriptionID)
	if err != nil {
		return err
	}

	subscription.Status = billingtypes.SubscriptionStatusActive
	subscription.RenewsAt = &nextBillingDate

	return s.repos.Subscription().Update(ctx, subscription)
}

// HandlePaymentFailed handles a failed payment
func (s *WebhookService) HandlePaymentFailed(ctx context.Context, providerSubscriptionID string) error {
	return s.repos.Subscription().UpdateStatusByProviderSubscriptionID(ctx, providerSubscriptionID, billingtypes.SubscriptionStatusPastDue)
}

// CreateOrder creates a new order from a payment webhook
func (s *WebhookService) CreateOrder(ctx context.Context, teamID string, payment *dodopayments.Payment) error {
	productID := ""
	if len(payment.ProductCart) > 0 {
		productID = payment.ProductCart[0].ProductID
	}

	subtotal := payment.TotalAmount - payment.Tax

	order := &models.Order{
		BillableType:    models.BillableTypeTeam,
		BillableID:      teamID,
		Provider:        models.ProviderDodoPayments,
		ProviderOrderID: payment.PaymentID,
		CustomerID:      payment.Customer.CustomerID,
		Identifier:      payment.PaymentID,
		ProductID:       productID,
		VariantID:       productID,
		OrderNumber:     0,
		Currency:        string(payment.Currency),
		Subtotal:        subtotal,
		DiscountTotal:   0,
		Tax:             payment.Tax,
		Total:           payment.TotalAmount,
		TaxName:         nil,
		Status:          billingtypes.OrderStatusPaid,
		ReceiptURL:      nil,
		Refunded:        false,
		OrderedAt:       payment.CreatedAt,
	}

	return s.repos.Order().Create(ctx, order)
}

// RefundOrder marks an order as refunded by payment ID
func (s *WebhookService) RefundOrder(ctx context.Context, paymentID string) error {
	order, err := s.repos.Order().FindByProviderOrderID(ctx, paymentID)
	if err != nil {
		return err
	}

	now := time.Now()
	order.Status = billingtypes.OrderStatusRefunded
	order.Refunded = true
	order.RefundedAt = &now

	return s.repos.Order().Update(ctx, order)
}

// HandleDisputeOpened handles a dispute being opened
func (s *WebhookService) HandleDisputeOpened(ctx context.Context, paymentID string) error {
	order, err := s.repos.Order().FindByProviderOrderID(ctx, paymentID)
	if err != nil {
		return err
	}

	order.Status = billingtypes.OrderStatusDisputed
	return s.repos.Order().Update(ctx, order)
}

// HandleDisputeResolved handles a dispute being resolved (won or lost)
func (s *WebhookService) HandleDisputeResolved(ctx context.Context, paymentID string, lost bool) error {
	order, err := s.repos.Order().FindByProviderOrderID(ctx, paymentID)
	if err != nil {
		return err
	}

	if lost {
		now := time.Now()
		order.Status = billingtypes.OrderStatusRefunded
		order.Refunded = true
		order.RefundedAt = &now
	} else {
		order.Status = billingtypes.OrderStatusPaid
	}

	return s.repos.Order().Update(ctx, order)
}

// MapDodoPaymentsStatus maps DodoPayments status to internal status
func MapDodoPaymentsStatus(status string) billingtypes.SubscriptionStatus {
	switch status {
	case "active":
		return billingtypes.SubscriptionStatusActive
	case "on_trial":
		return billingtypes.SubscriptionStatusOnTrial
	case "cancelled":
		return billingtypes.SubscriptionStatusCancelled
	case "expired":
		return billingtypes.SubscriptionStatusExpired
	case "failed":
		return billingtypes.SubscriptionStatusUnpaid
	case "on_hold":
		return billingtypes.SubscriptionStatusPaused
	case "renewed", "updated":
		return billingtypes.SubscriptionStatusActive
	default:
		return billingtypes.SubscriptionStatusActive
	}
}

// timeOrNil returns a pointer to t if it is not zero, otherwise nil
func timeOrNil(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}
