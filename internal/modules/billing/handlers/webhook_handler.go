package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"

	"github.com/kkz6/launch-go/internal/modules/billing/dto"
	"github.com/kkz6/launch-go/internal/modules/billing/models"
	"github.com/kkz6/launch-go/internal/modules/billing/services"
	billingtypes "github.com/kkz6/launch-go/internal/modules/billing/types"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/security"
)

// Webhook errors
var (
	ErrInvalidSignature     = errors.New("invalid webhook signature")
	ErrMissingSignature     = errors.New("missing webhook signature")
	ErrInvalidPayload       = errors.New("invalid webhook payload")
	ErrUnknownEventType     = errors.New("unknown webhook event type")
	ErrWebhookProcessFailed = errors.New("webhook processing failed")
)

// WebhookHandler handles incoming webhooks from DodoPayments
type WebhookHandler struct {
	logger         *zerolog.Logger
	webhookSecret  string
	service        *services.BillingService
	webhookService *services.WebhookService
	maxRetries     int
}

// NewWebhookHandler creates a new webhook handler
func NewWebhookHandler(service *services.BillingService, webhookService *services.WebhookService, webhookSecret string, logger *zerolog.Logger) *WebhookHandler {
	return &WebhookHandler{
		logger:         logger,
		webhookSecret:  webhookSecret,
		service:        service,
		webhookService: webhookService,
		maxRetries:     3,
	}
}

// HandleWebhook handles incoming webhook requests from DodoPayments
// DodoPayments uses Standard Webhooks spec with 3 headers:
// - webhook-id: Unique identifier for the webhook
// - webhook-signature: The signature in format "v1,base64signature"
// - webhook-timestamp: Unix timestamp when the webhook was sent
func (h *WebhookHandler) HandleWebhook(c *fiber.Ctx) error {
	webhookID := c.Get("webhook-id")
	signature := c.Get("webhook-signature")
	timestamp := c.Get("webhook-timestamp")

	if webhookID == "" || signature == "" || timestamp == "" {
		h.logger.Warn().
			Str("webhook_id", webhookID).
			Bool("has_signature", signature != "").
			Bool("has_timestamp", timestamp != "").
			Msg("Webhook received with missing headers")
		return fiberctx.RespondUnauthorized(c, "Missing required webhook headers")
	}

	body := c.Body()

	if !security.VerifyDodoPaymentsSignature(body, webhookID, signature, timestamp, h.webhookSecret) {
		h.logger.Warn().Str("webhook_id", webhookID).Msg("Invalid webhook signature")
		return fiberctx.RespondUnauthorized(c, "Invalid signature")
	}

	var payload dto.DodoWebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		h.logger.Error().Err(err).Msg("Failed to parse webhook payload")
		return fiberctx.RespondBadRequest(c, "Invalid payload")
	}

	event := &models.WebhookEvent{
		EventName: billingtypes.WebhookEventType(payload.Type),
		Payload:   string(body),
		Signature: signature,
		Processed: false,
	}

	if err := h.webhookService.CreateWebhookEvent(c.Context(), event); err != nil {
		h.logger.Error().Err(err).Msg("Failed to store webhook event")
		return fiberctx.RespondInternalError(c, "Failed to store event")
	}

	if err := h.processWebhook(c.Context(), event, &payload); err != nil {
		h.logger.Error().Err(err).Str("event_id", event.ID).Msg("Failed to process webhook")
		h.webhookService.MarkWebhookEventFailed(c.Context(), event.ID, err.Error())
		return fiberctx.OK(c, "Webhook received but processing failed", nil)
	}

	h.webhookService.MarkWebhookEventProcessed(c.Context(), event.ID)

	return fiberctx.OK(c, "Webhook processed successfully", nil)
}

// processWebhook processes a webhook event
func (h *WebhookHandler) processWebhook(ctx context.Context, event *models.WebhookEvent, payload *dto.DodoWebhookPayload) error {
	eventType := billingtypes.WebhookEventType(payload.Type)

	if !eventType.IsValid() {
		h.logger.Warn().Str("event", payload.Type).Msg("Unknown webhook event type")
		return nil
	}

	if eventType.IsSubscriptionEvent() {
		return h.handleSubscriptionEvent(ctx, eventType, payload)
	}

	if eventType.IsPaymentEvent() {
		return h.handlePaymentEvent(ctx, eventType, payload)
	}

	if eventType.IsRefundEvent() {
		return h.handleRefundEvent(ctx, eventType, payload)
	}

	if eventType.IsDisputeEvent() {
		return h.handleDisputeEvent(ctx, eventType, payload)
	}

	return nil
}

// handleSubscriptionEvent handles subscription-related webhook events
func (h *WebhookHandler) handleSubscriptionEvent(ctx context.Context, eventType billingtypes.WebhookEventType, payload *dto.DodoWebhookPayload) error {
	if payload.Data.Subscription == nil {
		return fmt.Errorf("missing subscription data in webhook payload")
	}

	sub := payload.Data.Subscription
	teamID := payload.GetTeamID()

	if teamID == "" {
		return fmt.Errorf("missing team_id in webhook metadata")
	}

	providerSubscriptionID := sub.SubscriptionID

	switch eventType {
	case billingtypes.WebhookEventSubscriptionActive:
		return h.webhookService.CreateOrUpdateSubscription(ctx, teamID, providerSubscriptionID, sub)

	case billingtypes.WebhookEventSubscriptionUpdated:
		return h.webhookService.UpdateSubscription(ctx, providerSubscriptionID, sub)

	case billingtypes.WebhookEventSubscriptionCancelled:
		return h.webhookService.CancelSubscriptionByWebhook(ctx, providerSubscriptionID, sub)

	case billingtypes.WebhookEventSubscriptionExpired:
		return h.webhookService.ExpireSubscription(ctx, providerSubscriptionID)

	case billingtypes.WebhookEventSubscriptionFailed:
		return h.webhookService.HandleSubscriptionFailed(ctx, providerSubscriptionID)

	case billingtypes.WebhookEventSubscriptionOnHold:
		return h.webhookService.PauseSubscription(ctx, providerSubscriptionID)

	case billingtypes.WebhookEventSubscriptionRenewed:
		return h.webhookService.HandleSubscriptionRenewed(ctx, providerSubscriptionID, sub)

	default:
		return nil
	}
}

// handlePaymentEvent handles payment-related webhook events
func (h *WebhookHandler) handlePaymentEvent(ctx context.Context, eventType billingtypes.WebhookEventType, payload *dto.DodoWebhookPayload) error {
	if payload.Data.Payment == nil {
		return fmt.Errorf("missing payment data in webhook payload")
	}

	payment := payload.Data.Payment
	teamID := payload.GetTeamID()

	if teamID == "" {
		return fmt.Errorf("missing team_id in webhook metadata")
	}

	switch eventType {
	case billingtypes.WebhookEventPaymentSucceeded:
		return h.webhookService.CreateOrder(ctx, teamID, payment)

	case billingtypes.WebhookEventPaymentFailed:
		if payment.SubscriptionID != nil {
			return h.webhookService.HandlePaymentFailed(ctx, *payment.SubscriptionID)
		}
		return nil

	case billingtypes.WebhookEventPaymentProcessing,
		billingtypes.WebhookEventPaymentCancelled:
		// Log but don't take action for these events
		h.logger.Info().Str("event", string(eventType)).Str("payment_id", payment.PaymentID).Msg("Payment event received")
		return nil

	default:
		return nil
	}
}

// handleRefundEvent handles refund-related webhook events
func (h *WebhookHandler) handleRefundEvent(ctx context.Context, eventType billingtypes.WebhookEventType, payload *dto.DodoWebhookPayload) error {
	if payload.Data.Refund == nil {
		return fmt.Errorf("missing refund data in webhook payload")
	}

	refund := payload.Data.Refund

	switch eventType {
	case billingtypes.WebhookEventRefundSucceeded:
		return h.webhookService.RefundOrder(ctx, refund.PaymentID)

	case billingtypes.WebhookEventRefundFailed:
		h.logger.Warn().Str("refund_id", refund.RefundID).Str("payment_id", refund.PaymentID).Msg("Refund failed")
		return nil

	default:
		return nil
	}
}

// handleDisputeEvent handles dispute-related webhook events
func (h *WebhookHandler) handleDisputeEvent(ctx context.Context, eventType billingtypes.WebhookEventType, payload *dto.DodoWebhookPayload) error {
	if payload.Data.Dispute == nil {
		return fmt.Errorf("missing dispute data in webhook payload")
	}

	dispute := payload.Data.Dispute

	switch eventType {
	case billingtypes.WebhookEventDisputeOpened:
		h.logger.Warn().
			Str("dispute_id", dispute.DisputeID).
			Str("payment_id", dispute.PaymentID).
			Int64("amount", dispute.Amount).
			Msg("Dispute opened")
		return h.webhookService.HandleDisputeOpened(ctx, dispute.PaymentID)

	case billingtypes.WebhookEventDisputeWon:
		h.logger.Info().Str("dispute_id", dispute.DisputeID).Msg("Dispute won")
		return h.webhookService.HandleDisputeResolved(ctx, dispute.PaymentID, false)

	case billingtypes.WebhookEventDisputeLost:
		h.logger.Warn().Str("dispute_id", dispute.DisputeID).Msg("Dispute lost")
		return h.webhookService.HandleDisputeResolved(ctx, dispute.PaymentID, true)

	default:
		return nil
	}
}

// ProcessPendingWebhooks processes any unprocessed webhook events
func (h *WebhookHandler) ProcessPendingWebhooks(ctx context.Context) error {
	events, err := h.webhookService.FindUnprocessedWebhookEvents(ctx, h.maxRetries)
	if err != nil {
		return err
	}

	for _, event := range events {
		var payload dto.DodoWebhookPayload
		if err := json.Unmarshal([]byte(event.Payload), &payload); err != nil {
			h.webhookService.MarkWebhookEventFailed(ctx, event.ID, err.Error())
			continue
		}

		if err := h.processWebhook(ctx, &event, &payload); err != nil {
			h.webhookService.MarkWebhookEventFailed(ctx, event.ID, err.Error())
			continue
		}

		h.webhookService.MarkWebhookEventProcessed(ctx, event.ID)
	}

	return nil
}

// CleanupOldWebhookEvents removes old processed webhook events
func (h *WebhookHandler) CleanupOldWebhookEvents(ctx context.Context, olderThan time.Duration) error {
	return h.webhookService.DeleteOldProcessedWebhookEvents(ctx, olderThan)
}
