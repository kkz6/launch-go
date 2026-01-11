package billing

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
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
	repo          *Repository
	service       *Service
	webhookSecret string
	logger        *zerolog.Logger
	maxRetries    int
}

// NewWebhookHandler creates a new webhook handler
func NewWebhookHandler(repo *Repository, service *Service, webhookSecret string, logger *zerolog.Logger) *WebhookHandler {
	return &WebhookHandler{
		repo:          repo,
		service:       service,
		webhookSecret: webhookSecret,
		logger:        logger,
		maxRetries:    3,
	}
}

// HandleWebhook handles incoming webhook requests
func (h *WebhookHandler) HandleWebhook(c *fiber.Ctx) error {
	signature := c.Get("X-Signature")
	if signature == "" {
		h.logger.Warn().Msg("Webhook received without signature")
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Missing signature",
		})
	}

	body := c.Body()

	if !h.verifySignature(body, signature) {
		h.logger.Warn().Msg("Invalid webhook signature")
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Invalid signature",
		})
	}

	var payload WebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		h.logger.Error().Err(err).Msg("Failed to parse webhook payload")
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid payload",
		})
	}

	event := &WebhookEvent{
		EventName: WebhookEventType(payload.Meta.EventName),
		Payload:   string(body),
		Signature: signature,
		Processed: false,
	}

	if err := h.repo.CreateWebhookEvent(c.Context(), event); err != nil {
		h.logger.Error().Err(err).Msg("Failed to store webhook event")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to store event",
		})
	}

	if err := h.processWebhook(c.Context(), event, &payload); err != nil {
		h.logger.Error().Err(err).Str("event_id", event.ID).Msg("Failed to process webhook")
		h.repo.MarkWebhookEventFailed(c.Context(), event.ID, err.Error())
		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"message": "Webhook received but processing failed",
			"error":   err.Error(),
		})
	}

	h.repo.MarkWebhookEventProcessed(c.Context(), event.ID)

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "Webhook processed successfully",
	})
}

// verifySignature verifies the webhook signature
func (h *WebhookHandler) verifySignature(payload []byte, signature string) bool {
	mac := hmac.New(sha256.New, []byte(h.webhookSecret))
	mac.Write(payload)
	expectedSignature := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(expectedSignature), []byte(signature))
}

// processWebhook processes a webhook event
func (h *WebhookHandler) processWebhook(ctx context.Context, event *WebhookEvent, payload *WebhookPayload) error {
	eventType := WebhookEventType(payload.Meta.EventName)

	if !eventType.IsValid() {
		h.logger.Warn().Str("event", payload.Meta.EventName).Msg("Unknown webhook event type")
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
func (h *WebhookHandler) handleSubscriptionEvent(ctx context.Context, eventType WebhookEventType, payload *WebhookPayload) error {
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
	case WebhookEventSubscriptionCreated:
		return h.handleSubscriptionCreated(ctx, teamID, lemonSqueezyID, &attrs)

	case WebhookEventSubscriptionUpdated:
		return h.handleSubscriptionUpdated(ctx, lemonSqueezyID, &attrs)

	case WebhookEventSubscriptionCancelled:
		return h.handleSubscriptionCancelled(ctx, lemonSqueezyID, &attrs)

	case WebhookEventSubscriptionResumed:
		return h.handleSubscriptionResumed(ctx, lemonSqueezyID, &attrs)

	case WebhookEventSubscriptionExpired:
		return h.handleSubscriptionExpired(ctx, lemonSqueezyID)

	case WebhookEventSubscriptionPaused:
		return h.handleSubscriptionPaused(ctx, lemonSqueezyID, &attrs)

	case WebhookEventSubscriptionUnpaused:
		return h.handleSubscriptionUnpaused(ctx, lemonSqueezyID)

	case WebhookEventSubscriptionPaymentSuccess:
		return h.handleSubscriptionPaymentSuccess(ctx, lemonSqueezyID, &attrs)

	case WebhookEventSubscriptionPaymentFailed:
		return h.handleSubscriptionPaymentFailed(ctx, lemonSqueezyID)

	case WebhookEventSubscriptionPaymentRecovered:
		return h.handleSubscriptionPaymentRecovered(ctx, lemonSqueezyID)

	default:
		return nil
	}
}

// handleSubscriptionCreated handles subscription_created event
func (h *WebhookHandler) handleSubscriptionCreated(ctx context.Context, teamID, lemonSqueezyID string, attrs *LemonSqueezyAttributes) error {
	status := mapLemonSqueezyStatus(attrs.Status)

	subscription := &Subscription{
		TeamID:         teamID,
		LemonSqueezyID: lemonSqueezyID,
		ProductID:      strconv.Itoa(attrs.ProductID),
		VariantID:      strconv.Itoa(attrs.VariantID),
		Name:           attrs.ProductName,
		Status:         status,
		CardBrand:      attrs.CardBrand,
		CardLastFour:   attrs.CardLastFour,
		BillingAnchor:  attrs.BillingAnchor,
		TrialEndsAt:    parseTime(attrs.TrialEndsAt),
		RenewsAt:       parseTime(attrs.RenewsAt),
		EndsAt:         parseTime(attrs.EndsAt),
	}

	if attrs.OrderID != nil {
		orderID := strconv.Itoa(*attrs.OrderID)
		subscription.OrderID = &orderID
	}

	return h.repo.CreateSubscription(ctx, subscription)
}

// handleSubscriptionUpdated handles subscription_updated event
func (h *WebhookHandler) handleSubscriptionUpdated(ctx context.Context, lemonSqueezyID string, attrs *LemonSqueezyAttributes) error {
	subscription, err := h.repo.FindSubscriptionByLemonSqueezyID(ctx, lemonSqueezyID)
	if err != nil {
		return err
	}

	subscription.ProductID = strconv.Itoa(attrs.ProductID)
	subscription.VariantID = strconv.Itoa(attrs.VariantID)
	subscription.Name = attrs.ProductName
	subscription.Status = mapLemonSqueezyStatus(attrs.Status)
	subscription.CardBrand = attrs.CardBrand
	subscription.CardLastFour = attrs.CardLastFour
	subscription.BillingAnchor = attrs.BillingAnchor
	subscription.TrialEndsAt = parseTime(attrs.TrialEndsAt)
	subscription.RenewsAt = parseTime(attrs.RenewsAt)
	subscription.EndsAt = parseTime(attrs.EndsAt)

	return h.repo.UpdateSubscription(ctx, subscription)
}

// handleSubscriptionCancelled handles subscription_cancelled event
func (h *WebhookHandler) handleSubscriptionCancelled(ctx context.Context, lemonSqueezyID string, attrs *LemonSqueezyAttributes) error {
	subscription, err := h.repo.FindSubscriptionByLemonSqueezyID(ctx, lemonSqueezyID)
	if err != nil {
		return err
	}

	subscription.Status = SubscriptionStatusCancelled
	subscription.EndsAt = parseTime(attrs.EndsAt)

	return h.repo.UpdateSubscription(ctx, subscription)
}

// handleSubscriptionResumed handles subscription_resumed event
func (h *WebhookHandler) handleSubscriptionResumed(ctx context.Context, lemonSqueezyID string, attrs *LemonSqueezyAttributes) error {
	subscription, err := h.repo.FindSubscriptionByLemonSqueezyID(ctx, lemonSqueezyID)
	if err != nil {
		return err
	}

	subscription.Status = SubscriptionStatusActive
	subscription.EndsAt = nil
	subscription.RenewsAt = parseTime(attrs.RenewsAt)

	return h.repo.UpdateSubscription(ctx, subscription)
}

// handleSubscriptionExpired handles subscription_expired event
func (h *WebhookHandler) handleSubscriptionExpired(ctx context.Context, lemonSqueezyID string) error {
	return h.repo.UpdateSubscriptionFields(ctx, lemonSqueezyID, map[string]interface{}{
		"status": SubscriptionStatusExpired,
	})
}

// handleSubscriptionPaused handles subscription_paused event
func (h *WebhookHandler) handleSubscriptionPaused(ctx context.Context, lemonSqueezyID string, attrs *LemonSqueezyAttributes) error {
	subscription, err := h.repo.FindSubscriptionByLemonSqueezyID(ctx, lemonSqueezyID)
	if err != nil {
		return err
	}

	now := time.Now()
	subscription.Status = SubscriptionStatusPaused
	subscription.PausedAt = &now
	subscription.ResumesAt = parseTime(attrs.ResumesAt)

	return h.repo.UpdateSubscription(ctx, subscription)
}

// handleSubscriptionUnpaused handles subscription_unpaused event
func (h *WebhookHandler) handleSubscriptionUnpaused(ctx context.Context, lemonSqueezyID string) error {
	subscription, err := h.repo.FindSubscriptionByLemonSqueezyID(ctx, lemonSqueezyID)
	if err != nil {
		return err
	}

	subscription.Status = SubscriptionStatusActive
	subscription.PausedAt = nil
	subscription.ResumesAt = nil

	return h.repo.UpdateSubscription(ctx, subscription)
}

// handleSubscriptionPaymentSuccess handles subscription_payment_success event
func (h *WebhookHandler) handleSubscriptionPaymentSuccess(ctx context.Context, lemonSqueezyID string, attrs *LemonSqueezyAttributes) error {
	subscription, err := h.repo.FindSubscriptionByLemonSqueezyID(ctx, lemonSqueezyID)
	if err != nil {
		return err
	}

	if subscription.Status == SubscriptionStatusPastDue || subscription.Status == SubscriptionStatusUnpaid {
		subscription.Status = SubscriptionStatusActive
	}

	subscription.RenewsAt = parseTime(attrs.RenewsAt)

	return h.repo.UpdateSubscription(ctx, subscription)
}

// handleSubscriptionPaymentFailed handles subscription_payment_failed event
func (h *WebhookHandler) handleSubscriptionPaymentFailed(ctx context.Context, lemonSqueezyID string) error {
	return h.repo.UpdateSubscriptionStatus(ctx, lemonSqueezyID, SubscriptionStatusPastDue)
}

// handleSubscriptionPaymentRecovered handles subscription_payment_recovered event
func (h *WebhookHandler) handleSubscriptionPaymentRecovered(ctx context.Context, lemonSqueezyID string) error {
	return h.repo.UpdateSubscriptionStatus(ctx, lemonSqueezyID, SubscriptionStatusActive)
}

// handleOrderEvent handles order-related webhook events
func (h *WebhookHandler) handleOrderEvent(ctx context.Context, eventType WebhookEventType, payload *WebhookPayload) error {
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
	case WebhookEventOrderCreated:
		return h.handleOrderCreated(ctx, teamID, lemonSqueezyID, &attrs)

	case WebhookEventOrderRefunded:
		return h.handleOrderRefunded(ctx, lemonSqueezyID)

	default:
		return nil
	}
}

// handleOrderCreated handles order_created event
func (h *WebhookHandler) handleOrderCreated(ctx context.Context, teamID, lemonSqueezyID string, attrs *LemonSqueezyAttributes) error {
	orderNumber := ""
	if attrs.OrderNumber != nil {
		orderNumber = strconv.Itoa(*attrs.OrderNumber)
	}

	currency := "USD"
	if attrs.Currency != nil {
		currency = *attrs.Currency
	}

	currencyRate := "1.0"
	if attrs.CurrencyRate != nil {
		currencyRate = *attrs.CurrencyRate
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

	order := &Order{
		TeamID:         teamID,
		LemonSqueezyID: lemonSqueezyID,
		CustomerID:     strconv.Itoa(attrs.CustomerID),
		ProductID:      strconv.Itoa(attrs.ProductID),
		VariantID:      strconv.Itoa(attrs.VariantID),
		OrderNumber:    orderNumber,
		Currency:       currency,
		CurrencyRate:   currencyRate,
		Subtotal:       subtotal,
		DiscountTotal:  discountTotal,
		Tax:            tax,
		Total:          total,
		TaxName:        attrs.TaxName,
		Status:         OrderStatusPaid,
		ReceiptURL:     attrs.ReceiptURL,
	}

	now := time.Now()
	order.OrderedAt = &now

	return h.repo.CreateOrder(ctx, order)
}

// handleOrderRefunded handles order_refunded event
func (h *WebhookHandler) handleOrderRefunded(ctx context.Context, lemonSqueezyID string) error {
	order, err := h.repo.FindOrderByLemonSqueezyID(ctx, lemonSqueezyID)
	if err != nil {
		return err
	}

	now := time.Now()
	order.Status = OrderStatusRefunded
	order.RefundedAt = &now

	return h.repo.UpdateOrder(ctx, order)
}

// ProcessPendingWebhooks processes any unprocessed webhook events
func (h *WebhookHandler) ProcessPendingWebhooks(ctx context.Context) error {
	events, err := h.repo.FindUnprocessedWebhookEvents(ctx, h.maxRetries)
	if err != nil {
		return err
	}

	for _, event := range events {
		var payload WebhookPayload
		if err := json.Unmarshal([]byte(event.Payload), &payload); err != nil {
			h.repo.MarkWebhookEventFailed(ctx, event.ID, err.Error())
			continue
		}

		if err := h.processWebhook(ctx, &event, &payload); err != nil {
			h.repo.MarkWebhookEventFailed(ctx, event.ID, err.Error())
			continue
		}

		h.repo.MarkWebhookEventProcessed(ctx, event.ID)
	}

	return nil
}

// CleanupOldWebhookEvents removes old processed webhook events
func (h *WebhookHandler) CleanupOldWebhookEvents(ctx context.Context, olderThan time.Duration) error {
	return h.repo.DeleteOldProcessedWebhookEvents(ctx, olderThan)
}

// mapLemonSqueezyStatus maps LemonSqueezy status to internal status
func mapLemonSqueezyStatus(status string) SubscriptionStatus {
	switch status {
	case "on_trial":
		return SubscriptionStatusOnTrial
	case "active":
		return SubscriptionStatusActive
	case "paused":
		return SubscriptionStatusPaused
	case "past_due":
		return SubscriptionStatusPastDue
	case "unpaid":
		return SubscriptionStatusUnpaid
	case "cancelled":
		return SubscriptionStatusCancelled
	case "expired":
		return SubscriptionStatusExpired
	default:
		return SubscriptionStatusActive
	}
}

// parseTime parses a time string from LemonSqueezy
func parseTime(s *string) *time.Time {
	if s == nil || *s == "" {
		return nil
	}

	t, err := time.Parse(time.RFC3339, *s)
	if err != nil {
		return nil
	}

	return &t
}
