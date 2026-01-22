package services

import (
	"context"
	"strconv"
	"time"

	"github.com/kkz6/launch-go/internal/modules/billing/dto"
	"github.com/kkz6/launch-go/internal/modules/billing/enums"
	"github.com/kkz6/launch-go/internal/modules/billing/models"
	"github.com/kkz6/launch-go/internal/modules/billing/repositories"
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

// CreateSubscription creates a new subscription
func (s *WebhookService) CreateSubscription(ctx context.Context, teamID, lemonSqueezyID string, attrs *dto.LemonSqueezyAttributes) error {
	status := MapLemonSqueezyStatus(attrs.Status)

	subscription := &models.Subscription{
		BillableType:   models.BillableTypeTeam,
		BillableID:     teamID,
		Type:           "default",
		LemonSqueezyID: lemonSqueezyID,
		ProductID:      strconv.Itoa(attrs.ProductID),
		VariantID:      strconv.Itoa(attrs.VariantID),
		Status:         status,
		CardBrand:      attrs.CardBrand,
		CardLastFour:   attrs.CardLastFour,
		TrialEndsAt:    ParseTime(attrs.TrialEndsAt),
		RenewsAt:       ParseTime(attrs.RenewsAt),
		EndsAt:         ParseTime(attrs.EndsAt),
	}

	return s.repos.Subscription().Create(ctx, subscription)
}

// UpdateSubscription updates an existing subscription
func (s *WebhookService) UpdateSubscription(ctx context.Context, lemonSqueezyID string, attrs *dto.LemonSqueezyAttributes) error {
	subscription, err := s.repos.Subscription().FindByLemonSqueezyID(ctx, lemonSqueezyID)
	if err != nil {
		return err
	}

	subscription.ProductID = strconv.Itoa(attrs.ProductID)
	subscription.VariantID = strconv.Itoa(attrs.VariantID)
	subscription.Status = MapLemonSqueezyStatus(attrs.Status)
	subscription.CardBrand = attrs.CardBrand
	subscription.CardLastFour = attrs.CardLastFour
	subscription.TrialEndsAt = ParseTime(attrs.TrialEndsAt)
	subscription.RenewsAt = ParseTime(attrs.RenewsAt)
	subscription.EndsAt = ParseTime(attrs.EndsAt)

	return s.repos.Subscription().Update(ctx, subscription)
}

// CancelSubscriptionByWebhook marks a subscription as cancelled from webhook
func (s *WebhookService) CancelSubscriptionByWebhook(ctx context.Context, lemonSqueezyID string, attrs *dto.LemonSqueezyAttributes) error {
	subscription, err := s.repos.Subscription().FindByLemonSqueezyID(ctx, lemonSqueezyID)
	if err != nil {
		return err
	}

	subscription.Status = enums.SubscriptionStatusCancelled
	subscription.EndsAt = ParseTime(attrs.EndsAt)

	return s.repos.Subscription().Update(ctx, subscription)
}

// ResumeSubscriptionByWebhook resumes a subscription from webhook
func (s *WebhookService) ResumeSubscriptionByWebhook(ctx context.Context, lemonSqueezyID string, attrs *dto.LemonSqueezyAttributes) error {
	subscription, err := s.repos.Subscription().FindByLemonSqueezyID(ctx, lemonSqueezyID)
	if err != nil {
		return err
	}

	subscription.Status = enums.SubscriptionStatusActive
	subscription.EndsAt = nil
	subscription.RenewsAt = ParseTime(attrs.RenewsAt)

	return s.repos.Subscription().Update(ctx, subscription)
}

// ExpireSubscription marks a subscription as expired
func (s *WebhookService) ExpireSubscription(ctx context.Context, lemonSqueezyID string) error {
	return s.repos.Subscription().UpdateStatusByLemonSqueezyID(ctx, lemonSqueezyID, enums.SubscriptionStatusExpired)
}

// PauseSubscription pauses a subscription
func (s *WebhookService) PauseSubscription(ctx context.Context, lemonSqueezyID string, attrs *dto.LemonSqueezyAttributes) error {
	subscription, err := s.repos.Subscription().FindByLemonSqueezyID(ctx, lemonSqueezyID)
	if err != nil {
		return err
	}

	pauseMode := "void"
	subscription.Status = enums.SubscriptionStatusPaused
	subscription.PauseMode = &pauseMode
	subscription.PauseResumesAt = ParseTime(attrs.ResumesAt)

	return s.repos.Subscription().Update(ctx, subscription)
}

// UnpauseSubscription unpauses a subscription
func (s *WebhookService) UnpauseSubscription(ctx context.Context, lemonSqueezyID string) error {
	subscription, err := s.repos.Subscription().FindByLemonSqueezyID(ctx, lemonSqueezyID)
	if err != nil {
		return err
	}

	subscription.Status = enums.SubscriptionStatusActive
	subscription.PauseMode = nil
	subscription.PauseResumesAt = nil

	return s.repos.Subscription().Update(ctx, subscription)
}

// HandlePaymentSuccess handles a successful payment
func (s *WebhookService) HandlePaymentSuccess(ctx context.Context, lemonSqueezyID string, attrs *dto.LemonSqueezyAttributes) error {
	subscription, err := s.repos.Subscription().FindByLemonSqueezyID(ctx, lemonSqueezyID)
	if err != nil {
		return err
	}

	if subscription.Status == enums.SubscriptionStatusPastDue || subscription.Status == enums.SubscriptionStatusUnpaid {
		subscription.Status = enums.SubscriptionStatusActive
	}

	subscription.RenewsAt = ParseTime(attrs.RenewsAt)

	return s.repos.Subscription().Update(ctx, subscription)
}

// HandlePaymentFailed handles a failed payment
func (s *WebhookService) HandlePaymentFailed(ctx context.Context, lemonSqueezyID string) error {
	return s.repos.Subscription().UpdateStatusByLemonSqueezyID(ctx, lemonSqueezyID, enums.SubscriptionStatusPastDue)
}

// HandlePaymentRecovered handles a recovered payment
func (s *WebhookService) HandlePaymentRecovered(ctx context.Context, lemonSqueezyID string) error {
	return s.repos.Subscription().UpdateStatusByLemonSqueezyID(ctx, lemonSqueezyID, enums.SubscriptionStatusActive)
}

// CreateOrder creates a new order
func (s *WebhookService) CreateOrder(ctx context.Context, teamID, lemonSqueezyID string, attrs *dto.LemonSqueezyAttributes) error {
	orderNumber := 0
	if attrs.OrderNumber != nil {
		orderNumber = *attrs.OrderNumber
	}

	currency := "USD"
	if attrs.Currency != nil {
		currency = *attrs.Currency
	}

	var subtotal, discountTotal, tax, total int64
	if attrs.Subtotal != nil {
		subtotal = *attrs.Subtotal
	}
	if attrs.DiscountTotal != nil {
		discountTotal = *attrs.DiscountTotal
	}
	if attrs.Tax != nil {
		tax = *attrs.Tax
	}
	if attrs.Total != nil {
		total = *attrs.Total
	}

	identifier := ""
	if attrs.Identifier != nil {
		identifier = *attrs.Identifier
	}

	order := &models.Order{
		BillableType:   models.BillableTypeTeam,
		BillableID:     teamID,
		LemonSqueezyID: lemonSqueezyID,
		CustomerID:     strconv.Itoa(attrs.CustomerID),
		Identifier:     identifier,
		ProductID:      strconv.Itoa(attrs.ProductID),
		VariantID:      strconv.Itoa(attrs.VariantID),
		OrderNumber:    orderNumber,
		Currency:       currency,
		Subtotal:       subtotal,
		DiscountTotal:  discountTotal,
		Tax:            tax,
		Total:          total,
		TaxName:        attrs.TaxName,
		Status:         enums.OrderStatusPaid,
		ReceiptURL:     attrs.ReceiptURL,
		Refunded:       false,
		OrderedAt:      time.Now(),
	}

	return s.repos.Order().Create(ctx, order)
}

// RefundOrder marks an order as refunded
func (s *WebhookService) RefundOrder(ctx context.Context, lemonSqueezyID string) error {
	order, err := s.repos.Order().FindByLemonSqueezyID(ctx, lemonSqueezyID)
	if err != nil {
		return err
	}

	now := time.Now()
	order.Status = enums.OrderStatusRefunded
	order.Refunded = true
	order.RefundedAt = &now

	return s.repos.Order().Update(ctx, order)
}

// MapLemonSqueezyStatus maps LemonSqueezy status to internal status
func MapLemonSqueezyStatus(status string) enums.SubscriptionStatus {
	switch status {
	case "on_trial":
		return enums.SubscriptionStatusOnTrial
	case "active":
		return enums.SubscriptionStatusActive
	case "paused":
		return enums.SubscriptionStatusPaused
	case "past_due":
		return enums.SubscriptionStatusPastDue
	case "unpaid":
		return enums.SubscriptionStatusUnpaid
	case "cancelled":
		return enums.SubscriptionStatusCancelled
	case "expired":
		return enums.SubscriptionStatusExpired
	default:
		return enums.SubscriptionStatusActive
	}
}

// ParseTime parses a time string from LemonSqueezy
func ParseTime(s *string) *time.Time {
	if s == nil || *s == "" {
		return nil
	}

	t, err := time.Parse(time.RFC3339, *s)
	if err != nil {
		return nil
	}

	return &t
}
