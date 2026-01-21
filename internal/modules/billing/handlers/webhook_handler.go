package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"

	"github.com/kkz6/launch-go/internal/modules/billing/dto"
	"github.com/kkz6/launch-go/internal/modules/billing/enums"
	"github.com/kkz6/launch-go/internal/modules/billing/models"
	"github.com/kkz6/launch-go/internal/modules/billing/repositories"
	"github.com/kkz6/launch-go/internal/modules/billing/services"
	"github.com/kkz6/launch-go/internal/pkg/response"
	"github.com/kkz6/launch-go/internal/pkg/webhook"
)

// Webhook errors
var (
	ErrInvalidSignature     = errors.New("invalid webhook signature")
	ErrMissingSignature     = errors.New("missing webhook signature")
	ErrInvalidPayload       = errors.New("invalid webhook payload")
	ErrUnknownEventType     = errors.New("unknown webhook event type")
	ErrWebhookProcessFailed = errors.New("webhook processing failed")
)

// WebhookHandler handles incoming webhooks from LemonSqueezy
type WebhookHandler struct {
	webhook.Base
	repos      *repositories.Registry
	service    *services.BillingService
	maxRetries int
}

// NewWebhookHandler creates a new webhook handler
func NewWebhookHandler(repos *repositories.Registry, service *services.BillingService, webhookSecret string, logger *zerolog.Logger) *WebhookHandler {
	return &WebhookHandler{
		Base:       webhook.NewBase(webhookSecret, logger),
		repos:      repos,
		service:    service,
		maxRetries: 3,
	}
}

// HandleWebhook handles incoming webhook requests
func (h *WebhookHandler) HandleWebhook(c *fiber.Ctx) error {
	signature := c.Get(webhook.HeaderLemonSqueezySignature)
	if signature == "" {
		h.LogWarn("Webhook received without signature")
		return response.Unauthorized(c, "Missing signature")
	}

	body := c.Body()

	if !h.verifyLemonSqueezySignature(body, signature) {
		h.LogWarn("Invalid webhook signature")
		return response.Unauthorized(c, "Invalid signature")
	}

	var payload dto.WebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		h.LogError(err, "Failed to parse webhook payload")
		return response.BadRequest(c, "Invalid payload")
	}

	event := &models.WebhookEvent{
		EventName: enums.WebhookEventType(payload.Meta.EventName),
		Payload:   string(body),
		Signature: signature,
		Processed: false,
	}

	if err := h.repos.WebhookEvent().Create(c.Context(), event); err != nil {
		h.LogError(err, "Failed to store webhook event")
		return response.InternalError(c, "Failed to store event")
	}

	if err := h.processWebhook(c.Context(), event, &payload); err != nil {
		h.LogError(err, "Failed to process webhook", "event_id", event.ID)
		h.repos.WebhookEvent().MarkFailed(c.Context(), event.ID, err.Error())
		return response.OK(c, "Webhook received but processing failed", nil)
	}

	h.repos.WebhookEvent().MarkProcessed(c.Context(), event.ID)

	return response.OK(c, "Webhook processed successfully", nil)
}

// verifyLemonSqueezySignature verifies the webhook signature using LemonSqueezy's HMAC format
func (h *WebhookHandler) verifyLemonSqueezySignature(payload []byte, signature string) bool {
	return webhook.VerifyHMACSHA256(payload, signature, h.Signer.GetSecretKey())
}

// processWebhook processes a webhook event
func (h *WebhookHandler) processWebhook(ctx context.Context, event *models.WebhookEvent, payload *dto.WebhookPayload) error {
	eventType := enums.WebhookEventType(payload.Meta.EventName)

	if !eventType.IsValid() {
		h.LogWarn("Unknown webhook event type", "event", payload.Meta.EventName)
		return nil
	}

	if eventType.IsSubscriptionEvent() {
		return h.handleSubscriptionEvent(ctx, eventType, payload)
	}

	if eventType.IsOrderEvent() {
		return h.handleOrderEvent(ctx, eventType, payload)
	}

	return nil
}

// handleSubscriptionEvent handles subscription-related webhook events
func (h *WebhookHandler) handleSubscriptionEvent(ctx context.Context, eventType enums.WebhookEventType, payload *dto.WebhookPayload) error {
	teamID := ""
	if payload.Meta.CustomData != nil {
		teamID = payload.Meta.CustomData["team_id"]
	}

	if teamID == "" {
		return fmt.Errorf("missing team_id in webhook custom data")
	}

	lemonSqueezyID := payload.Data.ID
	attrs := payload.Data.Attributes

	switch eventType {
	case enums.WebhookEventSubscriptionCreated:
		return h.handleSubscriptionCreated(ctx, teamID, lemonSqueezyID, &attrs)

	case enums.WebhookEventSubscriptionUpdated:
		return h.handleSubscriptionUpdated(ctx, lemonSqueezyID, &attrs)

	case enums.WebhookEventSubscriptionCancelled:
		return h.handleSubscriptionCancelled(ctx, lemonSqueezyID, &attrs)

	case enums.WebhookEventSubscriptionResumed:
		return h.handleSubscriptionResumed(ctx, lemonSqueezyID, &attrs)

	case enums.WebhookEventSubscriptionExpired:
		return h.handleSubscriptionExpired(ctx, lemonSqueezyID)

	case enums.WebhookEventSubscriptionPaused:
		return h.handleSubscriptionPaused(ctx, lemonSqueezyID, &attrs)

	case enums.WebhookEventSubscriptionUnpaused:
		return h.handleSubscriptionUnpaused(ctx, lemonSqueezyID)

	case enums.WebhookEventSubscriptionPaymentSuccess:
		return h.handleSubscriptionPaymentSuccess(ctx, lemonSqueezyID, &attrs)

	case enums.WebhookEventSubscriptionPaymentFailed:
		return h.handleSubscriptionPaymentFailed(ctx, lemonSqueezyID)

	case enums.WebhookEventSubscriptionPaymentRecovered:
		return h.handleSubscriptionPaymentRecovered(ctx, lemonSqueezyID)

	default:
		return nil
	}
}

// handleSubscriptionCreated handles subscription_created event
func (h *WebhookHandler) handleSubscriptionCreated(ctx context.Context, teamID, lemonSqueezyID string, attrs *dto.LemonSqueezyAttributes) error {
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

	return h.repos.Subscription().Create(ctx, subscription)
}

// handleSubscriptionUpdated handles subscription_updated event
func (h *WebhookHandler) handleSubscriptionUpdated(ctx context.Context, lemonSqueezyID string, attrs *dto.LemonSqueezyAttributes) error {
	subscription, err := h.repos.Subscription().FindByLemonSqueezyID(ctx, lemonSqueezyID)
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

	return h.repos.Subscription().Update(ctx, subscription)
}

// handleSubscriptionCancelled handles subscription_cancelled event
func (h *WebhookHandler) handleSubscriptionCancelled(ctx context.Context, lemonSqueezyID string, attrs *dto.LemonSqueezyAttributes) error {
	subscription, err := h.repos.Subscription().FindByLemonSqueezyID(ctx, lemonSqueezyID)
	if err != nil {
		return err
	}

	subscription.Status = enums.SubscriptionStatusCancelled
	subscription.EndsAt = ParseTime(attrs.EndsAt)

	return h.repos.Subscription().Update(ctx, subscription)
}

// handleSubscriptionResumed handles subscription_resumed event
func (h *WebhookHandler) handleSubscriptionResumed(ctx context.Context, lemonSqueezyID string, attrs *dto.LemonSqueezyAttributes) error {
	subscription, err := h.repos.Subscription().FindByLemonSqueezyID(ctx, lemonSqueezyID)
	if err != nil {
		return err
	}

	subscription.Status = enums.SubscriptionStatusActive
	subscription.EndsAt = nil
	subscription.RenewsAt = ParseTime(attrs.RenewsAt)

	return h.repos.Subscription().Update(ctx, subscription)
}

// handleSubscriptionExpired handles subscription_expired event
func (h *WebhookHandler) handleSubscriptionExpired(ctx context.Context, lemonSqueezyID string) error {
	return h.repos.Subscription().UpdateFields(ctx, lemonSqueezyID, map[string]interface{}{
		"status": enums.SubscriptionStatusExpired,
	})
}

// handleSubscriptionPaused handles subscription_paused event
func (h *WebhookHandler) handleSubscriptionPaused(ctx context.Context, lemonSqueezyID string, attrs *dto.LemonSqueezyAttributes) error {
	subscription, err := h.repos.Subscription().FindByLemonSqueezyID(ctx, lemonSqueezyID)
	if err != nil {
		return err
	}

	pauseMode := "void"
	subscription.Status = enums.SubscriptionStatusPaused
	subscription.PauseMode = &pauseMode
	subscription.PauseResumesAt = ParseTime(attrs.ResumesAt)

	return h.repos.Subscription().Update(ctx, subscription)
}

// handleSubscriptionUnpaused handles subscription_unpaused event
func (h *WebhookHandler) handleSubscriptionUnpaused(ctx context.Context, lemonSqueezyID string) error {
	subscription, err := h.repos.Subscription().FindByLemonSqueezyID(ctx, lemonSqueezyID)
	if err != nil {
		return err
	}

	subscription.Status = enums.SubscriptionStatusActive
	subscription.PauseMode = nil
	subscription.PauseResumesAt = nil

	return h.repos.Subscription().Update(ctx, subscription)
}

// handleSubscriptionPaymentSuccess handles subscription_payment_success event
func (h *WebhookHandler) handleSubscriptionPaymentSuccess(ctx context.Context, lemonSqueezyID string, attrs *dto.LemonSqueezyAttributes) error {
	subscription, err := h.repos.Subscription().FindByLemonSqueezyID(ctx, lemonSqueezyID)
	if err != nil {
		return err
	}

	if subscription.Status == enums.SubscriptionStatusPastDue || subscription.Status == enums.SubscriptionStatusUnpaid {
		subscription.Status = enums.SubscriptionStatusActive
	}

	subscription.RenewsAt = ParseTime(attrs.RenewsAt)

	return h.repos.Subscription().Update(ctx, subscription)
}

// handleSubscriptionPaymentFailed handles subscription_payment_failed event
func (h *WebhookHandler) handleSubscriptionPaymentFailed(ctx context.Context, lemonSqueezyID string) error {
	return h.repos.Subscription().UpdateStatus(ctx, lemonSqueezyID, enums.SubscriptionStatusPastDue)
}

// handleSubscriptionPaymentRecovered handles subscription_payment_recovered event
func (h *WebhookHandler) handleSubscriptionPaymentRecovered(ctx context.Context, lemonSqueezyID string) error {
	return h.repos.Subscription().UpdateStatus(ctx, lemonSqueezyID, enums.SubscriptionStatusActive)
}

// handleOrderEvent handles order-related webhook events
func (h *WebhookHandler) handleOrderEvent(ctx context.Context, eventType enums.WebhookEventType, payload *dto.WebhookPayload) error {
	teamID := ""
	if payload.Meta.CustomData != nil {
		teamID = payload.Meta.CustomData["team_id"]
	}

	if teamID == "" {
		return fmt.Errorf("missing team_id in webhook custom data")
	}

	lemonSqueezyID := payload.Data.ID
	attrs := payload.Data.Attributes

	switch eventType {
	case enums.WebhookEventOrderCreated:
		return h.handleOrderCreated(ctx, teamID, lemonSqueezyID, &attrs)

	case enums.WebhookEventOrderRefunded:
		return h.handleOrderRefunded(ctx, lemonSqueezyID)

	default:
		return nil
	}
}

// handleOrderCreated handles order_created event
func (h *WebhookHandler) handleOrderCreated(ctx context.Context, teamID, lemonSqueezyID string, attrs *dto.LemonSqueezyAttributes) error {
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

	return h.repos.Order().Create(ctx, order)
}

// handleOrderRefunded handles order_refunded event
func (h *WebhookHandler) handleOrderRefunded(ctx context.Context, lemonSqueezyID string) error {
	order, err := h.repos.Order().FindByLemonSqueezyID(ctx, lemonSqueezyID)
	if err != nil {
		return err
	}

	now := time.Now()
	order.Status = enums.OrderStatusRefunded
	order.Refunded = true
	order.RefundedAt = &now

	return h.repos.Order().Update(ctx, order)
}

// ProcessPendingWebhooks processes any unprocessed webhook events
func (h *WebhookHandler) ProcessPendingWebhooks(ctx context.Context) error {
	events, err := h.repos.WebhookEvent().FindUnprocessed(ctx, h.maxRetries)
	if err != nil {
		return err
	}

	for _, event := range events {
		var payload dto.WebhookPayload
		if err := json.Unmarshal([]byte(event.Payload), &payload); err != nil {
			h.repos.WebhookEvent().MarkFailed(ctx, event.ID, err.Error())
			continue
		}

		if err := h.processWebhook(ctx, &event, &payload); err != nil {
			h.repos.WebhookEvent().MarkFailed(ctx, event.ID, err.Error())
			continue
		}

		h.repos.WebhookEvent().MarkProcessed(ctx, event.ID)
	}

	return nil
}

// CleanupOldWebhookEvents removes old processed webhook events
func (h *WebhookHandler) CleanupOldWebhookEvents(ctx context.Context, olderThan time.Duration) error {
	return h.repos.WebhookEvent().DeleteOldProcessed(ctx, olderThan)
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
