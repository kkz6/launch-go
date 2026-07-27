package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

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

// WebhookHandler handles incoming webhooks from Polar.
type WebhookHandler struct {
	logger         *zerolog.Logger
	provider       *providers.PolarClient
	service        *services.BillingService
	webhookService *services.WebhookService
	maxRetries     int
}

// NewWebhookHandler creates a new webhook handler
func NewWebhookHandler(service *services.BillingService, webhookService *services.WebhookService, provider *providers.PolarClient, logger *zerolog.Logger) *WebhookHandler {
	return &WebhookHandler{
		logger:         logger,
		provider:       provider,
		service:        service,
		webhookService: webhookService,
		maxRetries:     3,
	}
}

// polarEvent is the envelope every Polar webhook shares: a type discriminator
// and the entity payload under `data`.
type polarEvent struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

// polarSubscription captures the subscription fields we persist. Unknown
// fields are ignored — Polar sends a much larger object.
type polarSubscription struct {
	ID                string            `json:"id"`
	Status            string            `json:"status"`
	CustomerID        string            `json:"customer_id"`
	ProductID         string            `json:"product_id"`
	CurrentPeriodEnd  *time.Time        `json:"current_period_end"`
	CancelAtPeriodEnd bool              `json:"cancel_at_period_end"`
	EndsAt            *time.Time        `json:"ends_at"`
	Metadata          map[string]string `json:"metadata"`
}

// polarOrder captures the order fields we persist.
type polarOrder struct {
	ID             string            `json:"id"`
	CustomerID     string            `json:"customer_id"`
	ProductID      string            `json:"product_id"`
	SubscriptionID string            `json:"subscription_id"`
	Currency       string            `json:"currency"`
	SubtotalAmount int64             `json:"subtotal_amount"`
	TaxAmount      int64             `json:"tax_amount"`
	TotalAmount    int64             `json:"total_amount"`
	CreatedAt      time.Time         `json:"created_at"`
	Metadata       map[string]string `json:"metadata"`
}

// HandleWebhook handles incoming Polar webhook requests. Polar uses the
// Standard Webhooks spec (webhook-id / webhook-signature / webhook-timestamp).
func (h *WebhookHandler) HandleWebhook(c *fiber.Ctx) error {
	if h.provider == nil {
		h.logger.Warn().Msg("Polar webhook received but provider is not configured")
		return fiberctx.RespondUnauthorized(c, "Billing provider not configured")
	}

	body := c.Body()

	headers := http.Header{}
	c.Request().Header.VisitAll(func(k, v []byte) {
		headers.Add(string(k), string(v))
	})

	if err := h.provider.ValidateWebhook(body, headers); err != nil {
		h.logger.Warn().Err(err).Msg("Polar webhook verification failed")
		return fiberctx.RespondUnauthorized(c, "Invalid signature")
	}

	var envelope polarEvent
	if err := json.Unmarshal(body, &envelope); err != nil {
		h.logger.Warn().Err(err).Msg("Failed to parse Polar webhook envelope")
		return fiberctx.RespondBadRequest(c, "Invalid payload")
	}

	eventType := billingtypes.WebhookEventType(envelope.Type)

	event := &models.WebhookEvent{
		EventName: eventType,
		Payload:   string(body),
		Signature: c.Get("webhook-signature"),
		Processed: false,
	}

	if err := h.webhookService.CreateWebhookEvent(c.Context(), event); err != nil {
		h.logger.Error().Err(err).Msg("Failed to store webhook event")
		return fiberctx.RespondInternalError(c, "Failed to store event")
	}

	if err := h.processEvent(c.Context(), envelope); err != nil {
		h.logger.Error().Err(err).Str("event_id", event.ID).Str("type", envelope.Type).Msg("Failed to process webhook")
		if markErr := h.webhookService.MarkWebhookEventFailed(c.Context(), event.ID, err.Error()); markErr != nil {
			h.logger.Error().Err(markErr).Str("event_id", event.ID).Msg("Failed to mark webhook event failed")
		}
		return fiberctx.OK(c, "Webhook received but processing failed", nil)
	}

	if markErr := h.webhookService.MarkWebhookEventProcessed(c.Context(), event.ID); markErr != nil {
		h.logger.Error().Err(markErr).Str("event_id", event.ID).Msg("Failed to mark webhook event processed")
		return fiberctx.RespondInternalError(c, "Failed to finalize webhook event")
	}

	return fiberctx.OK(c, "Webhook processed successfully", nil)
}

// processEvent dispatches a Polar webhook to the appropriate handler.
func (h *WebhookHandler) processEvent(ctx context.Context, e polarEvent) error {
	switch e.Type {
	case "subscription.created", "subscription.active", "subscription.uncanceled":
		sub, err := decodeSubscription(e.Data)
		if err != nil {
			return err
		}
		teamID := sub.Metadata["team_id"]
		if teamID == "" {
			return fmt.Errorf("missing team_id in subscription metadata")
		}
		return h.webhookService.CreateOrUpdateSubscription(ctx, teamID, toWebhookSubscription(sub))

	case "subscription.cycled":
		// Renewal (new billing period) on an already-stored subscription —
		// advance RenewsAt and keep it active without requiring team metadata.
		sub, err := decodeSubscription(e.Data)
		if err != nil {
			return err
		}
		return h.webhookService.HandleSubscriptionRenewed(ctx, sub.ID, sub.CurrentPeriodEnd)

	case "subscription.updated":
		sub, err := decodeSubscription(e.Data)
		if err != nil {
			return err
		}
		// A cancel-at-period-end update arrives as `subscription.updated`.
		if sub.CancelAtPeriodEnd {
			return h.webhookService.CancelSubscriptionByWebhook(ctx, sub.ID, periodEnd(sub))
		}
		return h.webhookService.UpdateSubscription(ctx, toWebhookSubscription(sub))

	case "subscription.canceled":
		sub, err := decodeSubscription(e.Data)
		if err != nil {
			return err
		}
		return h.webhookService.CancelSubscriptionByWebhook(ctx, sub.ID, periodEnd(sub))

	case "subscription.revoked":
		sub, err := decodeSubscription(e.Data)
		if err != nil {
			return err
		}
		return h.webhookService.ExpireSubscription(ctx, sub.ID)

	case "subscription.past_due":
		sub, err := decodeSubscription(e.Data)
		if err != nil {
			return err
		}
		return h.webhookService.HandleSubscriptionFailed(ctx, sub.ID)

	case "order.created", "order.paid":
		order, err := decodeOrder(e.Data)
		if err != nil {
			return err
		}
		teamID := order.Metadata["team_id"]
		if teamID == "" && order.SubscriptionID == "" {
			return fmt.Errorf("missing team_id in order metadata")
		}
		return h.webhookService.CreateOrder(ctx, teamID, toWebhookOrder(order))

	case "order.refunded":
		order, err := decodeOrder(e.Data)
		if err != nil {
			return err
		}
		return h.webhookService.RefundOrder(ctx, order.ID)

	default:
		// checkout.*, customer.*, benefit_grant.*, product.*, organization.* —
		// stored for audit, no state change.
		h.logger.Debug().Str("type", e.Type).Msg("Unhandled Polar webhook event")
		return nil
	}
}

func decodeSubscription(raw json.RawMessage) (polarSubscription, error) {
	var sub polarSubscription
	if err := json.Unmarshal(raw, &sub); err != nil {
		return polarSubscription{}, fmt.Errorf("decode subscription: %w", err)
	}
	return sub, nil
}

func decodeOrder(raw json.RawMessage) (polarOrder, error) {
	var order polarOrder
	if err := json.Unmarshal(raw, &order); err != nil {
		return polarOrder{}, fmt.Errorf("decode order: %w", err)
	}
	return order, nil
}

// periodEnd prefers an explicit ends_at, falling back to current_period_end.
func periodEnd(sub polarSubscription) *time.Time {
	if sub.EndsAt != nil {
		return sub.EndsAt
	}
	return sub.CurrentPeriodEnd
}

func toWebhookSubscription(sub polarSubscription) services.WebhookSubscription {
	return services.WebhookSubscription{
		ProviderSubscriptionID: sub.ID,
		CustomerID:             sub.CustomerID,
		ProductID:              sub.ProductID,
		Status:                 mapPolarStatus(sub.Status),
		RenewsAt:               sub.CurrentPeriodEnd,
		EndsAt:                 sub.EndsAt,
	}
}

func toWebhookOrder(order polarOrder) services.WebhookOrder {
	return services.WebhookOrder{
		ProviderOrderID: order.ID,
		CustomerID:      order.CustomerID,
		ProductID:       order.ProductID,
		SubscriptionID:  order.SubscriptionID,
		Currency:        order.Currency,
		Subtotal:        order.SubtotalAmount,
		Tax:             order.TaxAmount,
		Total:           order.TotalAmount,
		OrderedAt:       order.CreatedAt,
	}
}

// mapPolarStatus maps Polar subscription status to our internal status.
func mapPolarStatus(status string) billingtypes.SubscriptionStatus {
	switch status {
	case "active":
		return billingtypes.SubscriptionStatusActive
	case "trialing":
		return billingtypes.SubscriptionStatusOnTrial
	case "canceled":
		return billingtypes.SubscriptionStatusCancelled
	case "past_due":
		return billingtypes.SubscriptionStatusPastDue
	case "unpaid":
		return billingtypes.SubscriptionStatusUnpaid
	case "incomplete", "incomplete_expired":
		return billingtypes.SubscriptionStatusExpired
	default:
		// Fail closed: an unrecognized status must not grant access. Only
		// `active`/`on_trial` are access-granting, so default to a
		// non-granting state rather than silently unlocking features.
		return billingtypes.SubscriptionStatusPastDue
	}
}

// ProcessPendingWebhooks re-processes unprocessed webhook events from storage.
func (h *WebhookHandler) ProcessPendingWebhooks(ctx context.Context) error {
	events, err := h.webhookService.FindUnprocessedWebhookEvents(ctx, h.maxRetries)
	if err != nil {
		return err
	}

	for _, event := range events {
		var envelope polarEvent
		if err := json.Unmarshal([]byte(event.Payload), &envelope); err != nil {
			if markErr := h.webhookService.MarkWebhookEventFailed(ctx, event.ID, err.Error()); markErr != nil {
				h.logger.Error().Err(markErr).Str("event_id", event.ID).Msg("Failed to mark malformed webhook failed")
			}
			continue
		}

		if err := h.processEvent(ctx, envelope); err != nil {
			if markErr := h.webhookService.MarkWebhookEventFailed(ctx, event.ID, err.Error()); markErr != nil {
				h.logger.Error().Err(markErr).Str("event_id", event.ID).Msg("Failed to mark webhook processing failure")
			}
			continue
		}

		if markErr := h.webhookService.MarkWebhookEventProcessed(ctx, event.ID); markErr != nil {
			h.logger.Error().Err(markErr).Str("event_id", event.ID).Msg("Failed to mark pending webhook processed")
			return markErr
		}
	}

	return nil
}

// CleanupOldWebhookEvents removes old processed webhook events
func (h *WebhookHandler) CleanupOldWebhookEvents(ctx context.Context, olderThan time.Duration) error {
	return h.webhookService.DeleteOldProcessedWebhookEvents(ctx, olderThan)
}
