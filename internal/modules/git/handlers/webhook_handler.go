package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"

	"github.com/kkz6/launch-go/internal/modules/git/enums"
	"github.com/kkz6/launch-go/internal/modules/git/models"
	"github.com/kkz6/launch-go/internal/modules/git/providers"
	"github.com/kkz6/launch-go/internal/modules/git/services"
)

// WebhookHandler handles git webhooks
type WebhookHandler struct {
	service         *services.SourceControlService
	providerFactory *providers.ProviderFactory
	logger          *zerolog.Logger
}

// NewWebhookHandler creates a new webhook handler
func NewWebhookHandler(service *services.SourceControlService, providerFactory *providers.ProviderFactory, logger *zerolog.Logger) *WebhookHandler {
	return &WebhookHandler{
		service:         service,
		providerFactory: providerFactory,
		logger:          logger,
	}
}

// HandleWebhook handles incoming webhooks from git providers
func (h *WebhookHandler) HandleWebhook(c *fiber.Ctx) error {
	providerStr := c.Params("provider")

	providerType, err := enums.ParseGitProviderType(providerStr)
	if err != nil {
		h.logger.Error().Str("provider", providerStr).Msg("Invalid provider in webhook")
		return c.Status(fiber.StatusBadRequest).SendString("Invalid provider")
	}

	payload := c.Body()
	signature := h.getSignature(c, providerType)

	if signature == "" {
		h.logger.Warn().Str("provider", providerStr).Msg("Webhook received without signature")
		return c.Status(fiber.StatusBadRequest).SendString("Missing signature")
	}

	provider, err := h.providerFactory.GetProvider(providers.GitProviderType(providerType))
	if err != nil {
		h.logger.Error().Err(err).Str("provider", providerStr).Msg("Failed to get provider")
		return c.Status(fiber.StatusInternalServerError).SendString("Provider not configured")
	}

	if !provider.ValidateWebhook(payload, signature) {
		h.logger.Warn().Str("provider", providerStr).Msg("Webhook signature validation failed")
		return c.Status(fiber.StatusUnauthorized).SendString("Invalid signature")
	}

	// Parse payload
	var data map[string]interface{}
	if err := json.Unmarshal(payload, &data); err != nil {
		h.logger.Error().Err(err).Str("provider", providerStr).Msg("Failed to parse webhook payload")
		return c.Status(fiber.StatusBadRequest).SendString("Invalid JSON payload")
	}

	// Process webhook asynchronously
	go h.processWebhook(providerType, data, signature)

	h.logger.Info().Str("provider", providerStr).Msg("Webhook received and queued for processing")

	return c.Status(fiber.StatusOK).SendString("OK")
}

// getSignature extracts the webhook signature based on provider
func (h *WebhookHandler) getSignature(c *fiber.Ctx, providerType enums.GitProviderType) string {
	switch providerType {
	case enums.GitProviderGitHub:
		return c.Get("X-Hub-Signature-256")
	case enums.GitProviderGitLab:
		return c.Get("X-Gitlab-Token")
	case enums.GitProviderBitbucket:
		return c.Get("X-Hook-UUID")
	default:
		return ""
	}
}

// processWebhook processes a webhook payload
func (h *WebhookHandler) processWebhook(providerType enums.GitProviderType, data map[string]interface{}, signature string) {
	ctx := context.Background()

	switch providerType {
	case enums.GitProviderGitHub:
		h.processGitHubWebhook(ctx, data)
	case enums.GitProviderGitLab:
		h.processGitLabWebhook(ctx, data)
	case enums.GitProviderBitbucket:
		h.processBitbucketWebhook(ctx, data)
	}
}

// processGitHubWebhook processes GitHub webhook events
func (h *WebhookHandler) processGitHubWebhook(ctx context.Context, data map[string]interface{}) {
	action, _ := data["action"].(string)

	// Handle installation events
	if installation, ok := data["installation"].(map[string]interface{}); ok {
		var installationID string
		if idFloat, ok := installation["id"].(float64); ok {
			installationID = formatFloat(idFloat)
		}

		if installationID == "" {
			return
		}

		switch action {
		case "created":
			h.handleInstallationCreated(ctx, data, installationID)
		case "deleted":
			h.handleInstallationDeleted(ctx, installationID)
		case "repositories_added", "repositories_removed":
			h.handleRepositoriesChanged(ctx, installationID)
		}
	}

	// Handle push events for deployments
	if commits, ok := data["commits"].([]interface{}); ok && len(commits) > 0 {
		if repository, ok := data["repository"].(map[string]interface{}); ok {
			fullName, _ := repository["full_name"].(string)
			ref, _ := data["ref"].(string)
			branch := strings.TrimPrefix(ref, "refs/heads/")

			if fullName != "" && branch != "" {
				h.triggerDeployments(ctx, fullName, branch, data, enums.GitProviderGitHub)
			}
		}
	}
}

// processGitLabWebhook processes GitLab webhook events
func (h *WebhookHandler) processGitLabWebhook(ctx context.Context, data map[string]interface{}) {
	eventType, _ := data["event_type"].(string)

	if eventType == "push" {
		if project, ok := data["project"].(map[string]interface{}); ok {
			fullName, _ := project["path_with_namespace"].(string)
			ref, _ := data["ref"].(string)
			branch := strings.TrimPrefix(ref, "refs/heads/")

			if fullName != "" && branch != "" {
				h.triggerDeployments(ctx, fullName, branch, data, enums.GitProviderGitLab)
			}
		}
	}
}

// processBitbucketWebhook processes Bitbucket webhook events
func (h *WebhookHandler) processBitbucketWebhook(ctx context.Context, data map[string]interface{}) {
	if push, ok := data["push"].(map[string]interface{}); ok {
		if changes, ok := push["changes"].([]interface{}); ok && len(changes) > 0 {
			if repository, ok := data["repository"].(map[string]interface{}); ok {
				fullName, _ := repository["full_name"].(string)

				if change, ok := changes[0].(map[string]interface{}); ok {
					var branch string
					if newRef, ok := change["new"].(map[string]interface{}); ok {
						branch, _ = newRef["name"].(string)
					}

					if fullName != "" && branch != "" {
						h.triggerDeployments(ctx, fullName, branch, data, enums.GitProviderBitbucket)
					}
				}
			}
		}
	}
}

// handleInstallationCreated handles the creation of an app installation
func (h *WebhookHandler) handleInstallationCreated(ctx context.Context, data map[string]interface{}, installationID string) {
	sender, _ := data["sender"].(map[string]interface{})
	installation, _ := data["installation"].(map[string]interface{})

	if sender == nil || installation == nil {
		return
	}

	// Find existing source control
	sc, err := h.service.GetSourceControlByInstallation(ctx, enums.GitProviderGitHub, installationID)
	if err != nil {
		return
	}

	// Update with installer info
	providerData := sc.ProviderData
	if providerData == nil {
		providerData = make(models.JSONMap)
	}

	providerData["github_installer"] = map[string]interface{}{
		"login":                 sender["login"],
		"id":                    sender["id"],
		"type":                  sender["type"],
		"avatar_url":            sender["avatar_url"],
		"installed_via_webhook": true,
	}

	if err := h.service.GetSourceControlRepo().UpdateFields(ctx, sc.ID, map[string]interface{}{
		"provider_data": providerData,
	}); err != nil {
		h.logger.Error().Err(err).Str("source_control_id", sc.ID).Msg("Failed to update source control with installer info")
	}
}

// handleInstallationDeleted handles the deletion of an app installation
func (h *WebhookHandler) handleInstallationDeleted(ctx context.Context, installationID string) {
	if err := h.service.DeleteByInstallationID(ctx, installationID); err != nil {
		h.logger.Error().Err(err).Str("installation_id", installationID).Msg("Failed to delete source controls for installation")
	}
}

// handleRepositoriesChanged handles changes to repositories for an installation
func (h *WebhookHandler) handleRepositoriesChanged(ctx context.Context, installationID string) {
	if err := h.service.SyncRepositoriesForInstallation(ctx, installationID); err != nil {
		h.logger.Error().Err(err).Str("installation_id", installationID).Msg("Failed to sync repositories for installation")
	}
}

// triggerDeployments triggers deployments for sites matching the repository and branch
func (h *WebhookHandler) triggerDeployments(ctx context.Context, repository, branch string, webhookData map[string]interface{}, providerType enums.GitProviderType) {
	// TODO: Get sites by repository and branch from site repository
	// For now, just log the deployment trigger
	h.logger.Info().
		Str("repository", repository).
		Str("branch", branch).
		Str("provider", providerType.String()).
		Msg("Would trigger deployments for repository")

	// In the real implementation:
	// 1. Find all sites that match this repository and branch
	// 2. Get commit data from the webhook payload
	// 3. Trigger deployments for each site
}

// formatFloat formats a float64 as a string without decimal places
func formatFloat(f float64) string {
	return strings.TrimSuffix(
		strings.TrimSuffix(
			strings.Replace(
				strings.Replace(
					fmt.Sprintf("%f", f),
					".", "", -1),
				",", "", -1),
			"0"),
		".")
}
