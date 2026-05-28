package handlers

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dodopayments/dodopayments-go"
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"

	"github.com/kkz6/launch-go/internal/modules/billing/models"
	"github.com/kkz6/launch-go/internal/modules/billing/providers"
	"github.com/kkz6/launch-go/internal/modules/billing/services"
	billingtypes "github.com/kkz6/launch-go/internal/modules/billing/types"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
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
	dodoPayments   *providers.DodoPaymentsClient
	service        *services.BillingService
	webhookService *services.WebhookService
	maxRetries     int
}

// NewWebhookHandler creates a new webhook handler
func NewWebhookHandler(service *services.BillingService, webhookService *services.WebhookService, dodoPayments *providers.DodoPaymentsClient, logger *zerolog.Logger) *WebhookHandler {
	return &WebhookHandler{
		logger:         logger,
		dodoPayments:   dodoPayments,
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

	parsed, err := h.dodoPayments.UnwrapWebhook(body, webhookID, signature, timestamp)
	if err != nil {
		h.logger.Warn().Err(err).Str("webhook_id", webhookID).Msg("Webhook verification/parsing failed")
		return fiberctx.RespondUnauthorized(c, "Invalid signature")
	}

	eventType := billingtypes.WebhookEventType(parsed.Type)

	event := &models.WebhookEvent{
		EventName: eventType,
		Payload:   string(body),
		Signature: signature,
		Processed: false,
	}

	if err := h.webhookService.CreateWebhookEvent(c.Context(), event); err != nil {
		h.logger.Error().Err(err).Msg("Failed to store webhook event")
		return fiberctx.RespondInternalError(c, "Failed to store event")
	}

	if err := h.processEvent(c.Context(), eventType, parsed.AsUnion()); err != nil {
		h.logger.Error().Err(err).Str("event_id", event.ID).Msg("Failed to process webhook")
		_ = h.webhookService.MarkWebhookEventFailed(c.Context(), event.ID, err.Error())
		return fiberctx.OK(c, "Webhook received but processing failed", nil)
	}

	_ = h.webhookService.MarkWebhookEventProcessed(c.Context(), event.ID)

	return fiberctx.OK(c, "Webhook processed successfully", nil)
}

// processEvent dispatches a parsed webhook event to the appropriate handler
func (h *WebhookHandler) processEvent(ctx context.Context, eventType billingtypes.WebhookEventType, union dodopayments.UnwrapWebhookEventUnion) error {
	if !eventType.IsValid() {
		h.logger.Warn().Str("event", string(eventType)).Msg("Unknown webhook event type")
		return nil
	}

	switch e := union.(type) {
	// Subscription events
	case dodopayments.SubscriptionActiveWebhookEvent:
		teamID := e.Data.Metadata["team_id"]
		if teamID == "" {
			return fmt.Errorf("missing team_id in webhook metadata")
		}
		return h.webhookService.CreateOrUpdateSubscription(ctx, teamID, e.Data.SubscriptionID, &e.Data)

	case dodopayments.SubscriptionUpdatedWebhookEvent:
		return h.webhookService.UpdateSubscription(ctx, e.Data.SubscriptionID, &e.Data)

	case dodopayments.SubscriptionPlanChangedWebhookEvent:
		return h.webhookService.UpdateSubscription(ctx, e.Data.SubscriptionID, &e.Data)

	case dodopayments.SubscriptionCancelledWebhookEvent:
		return h.webhookService.CancelSubscriptionByWebhook(ctx, e.Data.SubscriptionID, e.Data.CancelledAt)

	case dodopayments.SubscriptionExpiredWebhookEvent:
		return h.webhookService.ExpireSubscription(ctx, e.Data.SubscriptionID)

	case dodopayments.SubscriptionFailedWebhookEvent:
		return h.webhookService.HandleSubscriptionFailed(ctx, e.Data.SubscriptionID)

	case dodopayments.SubscriptionOnHoldWebhookEvent:
		return h.webhookService.PauseSubscription(ctx, e.Data.SubscriptionID)

	case dodopayments.SubscriptionRenewedWebhookEvent:
		return h.webhookService.HandleSubscriptionRenewed(ctx, e.Data.SubscriptionID, e.Data.NextBillingDate)

	// Payment events
	case dodopayments.PaymentSucceededWebhookEvent:
		teamID := e.Data.Metadata["team_id"]
		if teamID == "" {
			return fmt.Errorf("missing team_id in webhook metadata")
		}
		return h.webhookService.CreateOrder(ctx, teamID, &e.Data)

	case dodopayments.PaymentFailedWebhookEvent:
		if e.Data.SubscriptionID != "" {
			return h.webhookService.HandlePaymentFailed(ctx, e.Data.SubscriptionID)
		}
		return nil

	case dodopayments.PaymentProcessingWebhookEvent:
		h.logger.Info().Str("event", string(eventType)).Str("payment_id", e.Data.PaymentID).Msg("Payment event received")
		return nil

	case dodopayments.PaymentCancelledWebhookEvent:
		h.logger.Info().Str("event", string(eventType)).Str("payment_id", e.Data.PaymentID).Msg("Payment event received")
		return nil

	// Refund events
	case dodopayments.RefundSucceededWebhookEvent:
		return h.webhookService.RefundOrder(ctx, e.Data.PaymentID)

	case dodopayments.RefundFailedWebhookEvent:
		h.logger.Warn().Str("refund_id", e.Data.RefundID).Str("payment_id", e.Data.PaymentID).Msg("Refund failed")
		return nil

	// Dispute events
	case dodopayments.DisputeOpenedWebhookEvent:
		h.logger.Warn().
			Str("dispute_id", e.Data.DisputeID).
			Str("payment_id", e.Data.PaymentID).
			Str("amount", e.Data.Amount).
			Msg("Dispute opened")
		return h.webhookService.HandleDisputeOpened(ctx, e.Data.PaymentID)

	case dodopayments.DisputeWonWebhookEvent:
		h.logger.Info().Str("dispute_id", e.Data.DisputeID).Msg("Dispute won")
		return h.webhookService.HandleDisputeResolved(ctx, e.Data.PaymentID, false)

	case dodopayments.DisputeLostWebhookEvent:
		h.logger.Warn().Str("dispute_id", e.Data.DisputeID).Msg("Dispute lost")
		return h.webhookService.HandleDisputeResolved(ctx, e.Data.PaymentID, true)

	case dodopayments.DisputeExpiredWebhookEvent:
		h.logger.Info().Str("dispute_id", e.Data.DisputeID).Msg("Dispute expired")
		return nil

	case dodopayments.DisputeAcceptedWebhookEvent:
		h.logger.Info().Str("dispute_id", e.Data.DisputeID).Msg("Dispute accepted")
		return nil

	case dodopayments.DisputeCancelledWebhookEvent:
		h.logger.Info().Str("dispute_id", e.Data.DisputeID).Msg("Dispute cancelled")
		return nil

	case dodopayments.DisputeChallengedWebhookEvent:
		h.logger.Info().Str("dispute_id", e.Data.DisputeID).Msg("Dispute challenged")
		return nil

	default:
		return nil
	}
}

// processEventFromUnsafe dispatches a parsed unsafe webhook event to the appropriate handler.
// Used for re-processing stored events that were already verified on receipt.
func (h *WebhookHandler) processEventFromUnsafe(ctx context.Context, eventType billingtypes.WebhookEventType, union dodopayments.UnsafeUnwrapWebhookEventUnion) error {
	if !eventType.IsValid() {
		h.logger.Warn().Str("event", string(eventType)).Msg("Unknown webhook event type")
		return nil
	}

	switch e := union.(type) {
	// Subscription events
	case dodopayments.SubscriptionActiveWebhookEvent:
		teamID := e.Data.Metadata["team_id"]
		if teamID == "" {
			return fmt.Errorf("missing team_id in webhook metadata")
		}
		return h.webhookService.CreateOrUpdateSubscription(ctx, teamID, e.Data.SubscriptionID, &e.Data)

	case dodopayments.SubscriptionUpdatedWebhookEvent:
		return h.webhookService.UpdateSubscription(ctx, e.Data.SubscriptionID, &e.Data)

	case dodopayments.SubscriptionPlanChangedWebhookEvent:
		return h.webhookService.UpdateSubscription(ctx, e.Data.SubscriptionID, &e.Data)

	case dodopayments.SubscriptionCancelledWebhookEvent:
		return h.webhookService.CancelSubscriptionByWebhook(ctx, e.Data.SubscriptionID, e.Data.CancelledAt)

	case dodopayments.SubscriptionExpiredWebhookEvent:
		return h.webhookService.ExpireSubscription(ctx, e.Data.SubscriptionID)

	case dodopayments.SubscriptionFailedWebhookEvent:
		return h.webhookService.HandleSubscriptionFailed(ctx, e.Data.SubscriptionID)

	case dodopayments.SubscriptionOnHoldWebhookEvent:
		return h.webhookService.PauseSubscription(ctx, e.Data.SubscriptionID)

	case dodopayments.SubscriptionRenewedWebhookEvent:
		return h.webhookService.HandleSubscriptionRenewed(ctx, e.Data.SubscriptionID, e.Data.NextBillingDate)

	// Payment events
	case dodopayments.PaymentSucceededWebhookEvent:
		teamID := e.Data.Metadata["team_id"]
		if teamID == "" {
			return fmt.Errorf("missing team_id in webhook metadata")
		}
		return h.webhookService.CreateOrder(ctx, teamID, &e.Data)

	case dodopayments.PaymentFailedWebhookEvent:
		if e.Data.SubscriptionID != "" {
			return h.webhookService.HandlePaymentFailed(ctx, e.Data.SubscriptionID)
		}
		return nil

	case dodopayments.PaymentProcessingWebhookEvent,
		dodopayments.PaymentCancelledWebhookEvent:
		return nil

	// Refund events
	case dodopayments.RefundSucceededWebhookEvent:
		return h.webhookService.RefundOrder(ctx, e.Data.PaymentID)

	case dodopayments.RefundFailedWebhookEvent:
		h.logger.Warn().Str("refund_id", e.Data.RefundID).Str("payment_id", e.Data.PaymentID).Msg("Refund failed")
		return nil

	// Dispute events
	case dodopayments.DisputeOpenedWebhookEvent:
		return h.webhookService.HandleDisputeOpened(ctx, e.Data.PaymentID)

	case dodopayments.DisputeWonWebhookEvent:
		return h.webhookService.HandleDisputeResolved(ctx, e.Data.PaymentID, false)

	case dodopayments.DisputeLostWebhookEvent:
		return h.webhookService.HandleDisputeResolved(ctx, e.Data.PaymentID, true)

	case dodopayments.DisputeExpiredWebhookEvent,
		dodopayments.DisputeAcceptedWebhookEvent,
		dodopayments.DisputeCancelledWebhookEvent,
		dodopayments.DisputeChallengedWebhookEvent:
		return nil

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
		parsed, err := h.dodoPayments.UnsafeUnwrapWebhook([]byte(event.Payload))
		if err != nil {
			_ = h.webhookService.MarkWebhookEventFailed(ctx, event.ID, err.Error())
			continue
		}

		eventType := billingtypes.WebhookEventType(parsed.Type)

		if err := h.processEventFromUnsafe(ctx, eventType, parsed.AsUnion()); err != nil {
			_ = h.webhookService.MarkWebhookEventFailed(ctx, event.ID, err.Error())
			continue
		}

		_ = h.webhookService.MarkWebhookEventProcessed(ctx, event.ID)
	}

	return nil
}

// CleanupOldWebhookEvents removes old processed webhook events
func (h *WebhookHandler) CleanupOldWebhookEvents(ctx context.Context, olderThan time.Duration) error {
	return h.webhookService.DeleteOldProcessedWebhookEvents(ctx, olderThan)
}
