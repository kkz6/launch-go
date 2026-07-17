package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"

	"github.com/kkz6/launch-go/internal/modules/git/jobs"
	"github.com/kkz6/launch-go/internal/modules/git/providers"
	gittypes "github.com/kkz6/launch-go/internal/modules/git/types"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// WebhookHandler validates and queues git provider webhooks.
type WebhookHandler struct {
	logger          *zerolog.Logger
	providerFactory *providers.ProviderFactory
	queueClient     taskrunner.QueueClient
}

// NewWebhookHandler creates a new webhook handler.
func NewWebhookHandler(providerFactory *providers.ProviderFactory, logger *zerolog.Logger) *WebhookHandler {
	return &WebhookHandler{
		logger:          logger,
		providerFactory: providerFactory,
	}
}

// SetQueueClient sets the queue client for async processing.
func (h *WebhookHandler) SetQueueClient(client taskrunner.QueueClient) {
	h.queueClient = client
}

// HandleWebhook validates and queues an incoming provider webhook.
func (h *WebhookHandler) HandleWebhook(c *fiber.Ctx) error {
	providerName := c.Params("provider")
	providerType, err := gittypes.ParseGitProviderType(providerName)
	if err != nil {
		h.logger.Error().Err(err).Str("provider", providerName).Msg("Invalid provider in webhook")

		return fiberctx.RespondBadRequest(c, "Invalid provider")
	}

	payload := c.Body()
	signature := webhookSignature(c, providerType)
	if signature == "" {
		h.logger.Warn().Str("provider", providerName).Msg("Webhook received without signature")

		return fiberctx.RespondBadRequest(c, "Missing signature")
	}

	provider, err := h.providerFactory.GetProvider(providers.GitProviderType(providerType))
	if err != nil {
		h.logger.Error().Err(err).Str("provider", providerName).Msg("Failed to get provider")

		return fiberctx.RespondInternalError(c, "Provider not configured")
	}

	if !provider.ValidateWebhook(payload, signature) {
		h.logger.Warn().Str("provider", providerName).Msg("Webhook signature validation failed")

		return fiberctx.RespondUnauthorized(c, "Invalid signature")
	}

	if h.queueClient == nil {
		h.logger.Error().Str("provider", providerName).Msg("Webhook queue is unavailable")

		return fiberctx.RespondInternalError(c, "Webhook queue unavailable")
	}

	task, err := jobs.NewProcessGitWebhookTask(providerName, string(payload), signature)
	if err != nil {
		h.logger.Error().Err(err).Str("provider", providerName).Msg("Failed to create webhook task")

		return fiberctx.RespondInternalError(c, "Failed to queue webhook")
	}

	if _, err := h.queueClient.Enqueue(task); err != nil {
		h.logger.Error().Err(err).Str("provider", providerName).Msg("Failed to enqueue webhook task")

		return fiberctx.RespondInternalError(c, "Failed to queue webhook")
	}

	h.logger.Info().Str("provider", providerName).Msg("Webhook queued for async processing")

	return fiberctx.OK(c, "OK", nil)
}

func webhookSignature(c *fiber.Ctx, providerType gittypes.GitProviderType) string {
	switch providerType {
	case gittypes.GitProviderGitHub:
		return c.Get("X-Hub-Signature-256")
	case gittypes.GitProviderGitLab:
		return c.Get("X-Gitlab-Token")
	case gittypes.GitProviderBitbucket:
		return c.Get("X-Hub-Signature")
	default:
		return ""
	}
}
