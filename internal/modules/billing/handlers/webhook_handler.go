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
	service        *services.BillingService
	webhookService *services.WebhookService
	maxRetries     int
}

// NewWebhookHandler creates a new webhook handler
func NewWebhookHandler(service *services.BillingService, webhookService *services.WebhookService, webhookSecret string, logger *zerolog.Logger) *WebhookHandler {
	return &WebhookHandler{
		Base:           webhook.NewBase(webhookSecret, logger),
		service:        service,
		webhookService: webhookService,
		maxRetries:     3,
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
		EventName: billingtypes.WebhookEventType(payload.Meta.EventName),
		Payload:   string(body),
		Signature: signature,
		Processed: false,
	}

	if err := h.webhookService.CreateWebhookEvent(c.Context(), event); err != nil {
		h.LogError(err, "Failed to store webhook event")
		return response.InternalError(c, "Failed to store event")
	}

	if err := h.processWebhook(c.Context(), event, &payload); err != nil {
		h.LogError(err, "Failed to process webhook", "event_id", event.ID)
		h.webhookService.MarkWebhookEventFailed(c.Context(), event.ID, err.Error())
		return response.OK(c, "Webhook received but processing failed", nil)
	}

	h.webhookService.MarkWebhookEventProcessed(c.Context(), event.ID)

	return response.OK(c, "Webhook processed successfully", nil)
}

// verifyLemonSqueezySignature verifies the webhook signature using LemonSqueezy's HMAC format
func (h *WebhookHandler) verifyLemonSqueezySignature(payload []byte, signature string) bool {
	return webhook.VerifyHMACSHA256(payload, signature, h.Signer.GetSecretKey())
}

// processWebhook processes a webhook event
func (h *WebhookHandler) processWebhook(ctx context.Context, event *models.WebhookEvent, payload *dto.WebhookPayload) error {
	eventType := billingtypes.WebhookEventType(payload.Meta.EventName)

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
func (h *WebhookHandler) handleSubscriptionEvent(ctx context.Context, eventType billingtypes.WebhookEventType, payload *dto.WebhookPayload) error {
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
	case billingtypes.WebhookEventSubscriptionCreated:
		return h.webhookService.CreateSubscription(ctx, teamID, lemonSqueezyID, &attrs)

	case billingtypes.WebhookEventSubscriptionUpdated:
		return h.webhookService.UpdateSubscription(ctx, lemonSqueezyID, &attrs)

	case billingtypes.WebhookEventSubscriptionCancelled:
		return h.webhookService.CancelSubscriptionByWebhook(ctx, lemonSqueezyID, &attrs)

	case billingtypes.WebhookEventSubscriptionResumed:
		return h.webhookService.ResumeSubscriptionByWebhook(ctx, lemonSqueezyID, &attrs)

	case billingtypes.WebhookEventSubscriptionExpired:
		return h.webhookService.ExpireSubscription(ctx, lemonSqueezyID)

	case billingtypes.WebhookEventSubscriptionPaused:
		return h.webhookService.PauseSubscription(ctx, lemonSqueezyID, &attrs)

	case billingtypes.WebhookEventSubscriptionUnpaused:
		return h.webhookService.UnpauseSubscription(ctx, lemonSqueezyID)

	case billingtypes.WebhookEventSubscriptionPaymentSuccess:
		return h.webhookService.HandlePaymentSuccess(ctx, lemonSqueezyID, &attrs)

	case billingtypes.WebhookEventSubscriptionPaymentFailed:
		return h.webhookService.HandlePaymentFailed(ctx, lemonSqueezyID)

	case billingtypes.WebhookEventSubscriptionPaymentRecovered:
		return h.webhookService.HandlePaymentRecovered(ctx, lemonSqueezyID)

	default:
		return nil
	}
}

// handleOrderEvent handles order-related webhook events
func (h *WebhookHandler) handleOrderEvent(ctx context.Context, eventType billingtypes.WebhookEventType, payload *dto.WebhookPayload) error {
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
	case billingtypes.WebhookEventOrderCreated:
		return h.webhookService.CreateOrder(ctx, teamID, lemonSqueezyID, &attrs)

	case billingtypes.WebhookEventOrderRefunded:
		return h.webhookService.RefundOrder(ctx, lemonSqueezyID)

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
		var payload dto.WebhookPayload
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
