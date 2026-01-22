package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"

	"github.com/kkz6/launch-go/internal/modules/git/gitref"
	"github.com/kkz6/launch-go/internal/modules/git/jobs"
	"github.com/kkz6/launch-go/internal/modules/git/providers"
	"github.com/kkz6/launch-go/internal/modules/git/services"
	gittypes "github.com/kkz6/launch-go/internal/modules/git/types"
	"github.com/kkz6/launch-go/internal/pkg/response"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
	"github.com/kkz6/launch-go/internal/pkg/webhook"
)

// WebhookHandler handles git webhooks
type WebhookHandler struct {
	webhook.Base
	service         *services.SourceControlService
	providerFactory *providers.ProviderFactory
	queueClient     taskrunner.QueueClient
}

// NewWebhookHandler creates a new webhook handler
func NewWebhookHandler(service *services.SourceControlService, providerFactory *providers.ProviderFactory, logger *zerolog.Logger) *WebhookHandler {
	return &WebhookHandler{
		Base:            webhook.NewBase("", logger), // Git webhooks use provider-specific signature validation
		service:         service,
		providerFactory: providerFactory,
	}
}

// SetQueueClient sets the queue client for async processing
func (h *WebhookHandler) SetQueueClient(client taskrunner.QueueClient) {
	h.queueClient = client
}

// HandleWebhook handles incoming webhooks from git providers
func (h *WebhookHandler) HandleWebhook(c *fiber.Ctx) error {
	providerStr := c.Params("provider")

	providerType, err := gittypes.ParseGitProviderType(providerStr)
	if err != nil {
		h.LogError(err, "Invalid provider in webhook", "provider", providerStr)
		return response.BadRequest(c, "Invalid provider")
	}

	payload := c.Body()
	signature := h.getSignature(c, providerType)

	if signature == "" {
		h.LogWarn("Webhook received without signature", "provider", providerStr)
		return response.BadRequest(c, "Missing signature")
	}

	provider, err := h.providerFactory.GetProvider(providers.GitProviderType(providerType))
	if err != nil {
		h.LogError(err, "Failed to get provider", "provider", providerStr)
		return response.InternalError(c, "Provider not configured")
	}

	if !provider.ValidateWebhook(payload, signature) {
		h.LogWarn("Webhook signature validation failed", "provider", providerStr)
		return response.Unauthorized(c, "Invalid signature")
	}

	// Dispatch job for async processing
	if h.queueClient != nil {
		task, err := jobs.NewProcessGitWebhookTask(providerStr, string(payload), signature)
		if err != nil {
			h.LogError(err, "Failed to create webhook task")
		} else if _, err := h.queueClient.Enqueue(task); err != nil {
			h.LogError(err, "Failed to enqueue webhook task")
		} else {
			h.LogInfo("Webhook queued for async processing", "provider", providerStr)
			return response.OK(c, "OK", nil)
		}
	}

	// Fallback: Process synchronously if no queue
	var data map[string]interface{}
	if err := json.Unmarshal(payload, &data); err != nil {
		h.LogError(err, "Failed to parse webhook payload", "provider", providerStr)
		return response.BadRequest(c, "Invalid JSON payload")
	}

	// Process webhook in goroutine as fallback
	go h.processWebhook(providerType, data, signature)

	h.LogInfo("Webhook received and processing", "provider", providerStr)

	return response.OK(c, "OK", nil)
}

// getSignature extracts the webhook signature based on provider
func (h *WebhookHandler) getSignature(c *fiber.Ctx, providerType gittypes.GitProviderType) string {
	switch providerType {
	case gittypes.GitProviderGitHub:
		return c.Get("X-Hub-Signature-256")
	case gittypes.GitProviderGitLab:
		return c.Get("X-Gitlab-Token")
	case gittypes.GitProviderBitbucket:
		return c.Get("X-Hook-UUID")
	default:
		return ""
	}
}

// processWebhook processes a webhook payload
func (h *WebhookHandler) processWebhook(providerType gittypes.GitProviderType, data map[string]interface{}, signature string) {
	ctx := context.Background()

	switch providerType {
	case gittypes.GitProviderGitHub:
		h.processGitHubWebhook(ctx, data)
	case gittypes.GitProviderGitLab:
		h.processGitLabWebhook(ctx, data)
	case gittypes.GitProviderBitbucket:
		h.processBitbucketWebhook(ctx, data)
	}
}

// processGitHubWebhook processes GitHub webhook events
func (h *WebhookHandler) processGitHubWebhook(ctx context.Context, data map[string]interface{}) {
	action, ok := data["action"].(string)
	if !ok && data["action"] != nil {
		h.LogDebug("GitHub webhook: unexpected action type", "action", data["action"])
	}

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
			fullName, ok := repository["full_name"].(string)
			if !ok {
				h.LogDebug("GitHub webhook: unexpected full_name type", "full_name", repository["full_name"])
			}
			ref, ok := data["ref"].(string)
			if !ok {
				h.LogDebug("GitHub webhook: unexpected ref type", "ref", data["ref"])
			}
			branch := gitref.ExtractBranchName(ref)

			if fullName != "" && branch != "" {
				h.triggerDeployments(ctx, fullName, branch, data, gittypes.GitProviderGitHub)
			}
		}
	}
}

// processGitLabWebhook processes GitLab webhook events
func (h *WebhookHandler) processGitLabWebhook(ctx context.Context, data map[string]interface{}) {
	eventType, ok := data["event_type"].(string)
	if !ok && data["event_type"] != nil {
		h.LogDebug("GitLab webhook: unexpected event_type type", "event_type", data["event_type"])
	}

	if eventType == "push" {
		if project, ok := data["project"].(map[string]interface{}); ok {
			fullName, ok := project["path_with_namespace"].(string)
			if !ok {
				h.LogDebug("GitLab webhook: unexpected path_with_namespace type", "path_with_namespace", project["path_with_namespace"])
			}
			ref, ok := data["ref"].(string)
			if !ok {
				h.LogDebug("GitLab webhook: unexpected ref type", "ref", data["ref"])
			}
			branch := gitref.ExtractBranchName(ref)

			if fullName != "" && branch != "" {
				h.triggerDeployments(ctx, fullName, branch, data, gittypes.GitProviderGitLab)
			}
		}
	}
}

// processBitbucketWebhook processes Bitbucket webhook events
func (h *WebhookHandler) processBitbucketWebhook(ctx context.Context, data map[string]interface{}) {
	if push, ok := data["push"].(map[string]interface{}); ok {
		if changes, ok := push["changes"].([]interface{}); ok && len(changes) > 0 {
			if repository, ok := data["repository"].(map[string]interface{}); ok {
				fullName, ok := repository["full_name"].(string)
				if !ok {
					h.LogDebug("Bitbucket webhook: unexpected full_name type", "full_name", repository["full_name"])
				}

				if change, ok := changes[0].(map[string]interface{}); ok {
					var branch string
					if newRef, ok := change["new"].(map[string]interface{}); ok {
						branch, ok = newRef["name"].(string)
						if !ok {
							h.LogDebug("Bitbucket webhook: unexpected branch name type", "name", newRef["name"])
						}
					}

					if fullName != "" && branch != "" {
						h.triggerDeployments(ctx, fullName, branch, data, gittypes.GitProviderBitbucket)
					}
				}
			}
		}
	}
}

// handleInstallationCreated handles the creation of an app installation
func (h *WebhookHandler) handleInstallationCreated(ctx context.Context, data map[string]interface{}, installationID string) {
	sender, ok := data["sender"].(map[string]interface{})
	if !ok {
		h.LogDebug("GitHub webhook: unexpected sender type in installation created", "sender", data["sender"])
		return
	}
	installation, ok := data["installation"].(map[string]interface{})
	if !ok {
		h.LogDebug("GitHub webhook: unexpected installation type in installation created", "installation", data["installation"])
		return
	}

	if sender == nil || installation == nil {
		return
	}

	// Find existing source control
	sc, err := h.service.GetSourceControlByInstallation(ctx, gittypes.GitProviderGitHub, installationID)
	if err != nil {
		return
	}

	// Update with installer info
	providerData := make(map[string]interface{})
	if sc.ProviderData != nil && *sc.ProviderData != "" {
		if err := json.Unmarshal([]byte(*sc.ProviderData), &providerData); err != nil {
			h.LogWarn("Failed to parse existing provider data", "error", err.Error())
		}
	}

	providerData["github_installer"] = map[string]interface{}{
		"login":                 sender["login"],
		"id":                    sender["id"],
		"type":                  sender["type"],
		"avatar_url":            sender["avatar_url"],
		"installed_via_webhook": true,
	}

	providerDataJSON, err := json.Marshal(providerData)
	if err != nil {
		h.LogError(err, "Failed to marshal provider data")
		return
	}
	providerDataStr := string(providerDataJSON)

	if err := h.service.GetSourceControlRepo().UpdateFields(ctx, sc.ID, map[string]interface{}{
		"provider_data": providerDataStr,
	}); err != nil {
		h.LogError(err, "Failed to update source control with installer info", "source_control_id", sc.ID)
	}
}

// handleInstallationDeleted handles the deletion of an app installation
func (h *WebhookHandler) handleInstallationDeleted(ctx context.Context, installationID string) {
	if err := h.service.DeleteByInstallationID(ctx, installationID); err != nil {
		h.LogError(err, "Failed to delete source controls for installation", "installation_id", installationID)
	}
}

// handleRepositoriesChanged handles changes to repositories for an installation
func (h *WebhookHandler) handleRepositoriesChanged(ctx context.Context, installationID string) {
	// Dispatch SyncInstallationRepos job via queue for async processing
	if h.queueClient != nil {
		// Get the source control to get team and user IDs
		sc, err := h.service.GetSourceControlByInstallation(ctx, gittypes.GitProviderGitHub, installationID)
		if err != nil {
			h.LogError(err, "Failed to find source control for installation", "installation_id", installationID)
			return
		}

		task, err := jobs.NewSyncInstallationReposTask(
			string(gittypes.GitProviderGitHub),
			installationID,
			sc.TeamID,
			sc.UserID,
		)
		if err != nil {
			h.LogError(err, "Failed to create sync installation repos task")
		} else if _, err := h.queueClient.Enqueue(task); err != nil {
			h.LogError(err, "Failed to enqueue sync installation repos task")
		} else {
			h.LogInfo("Sync installation repos job queued", "installation_id", installationID)
			return
		}
	}

	// Fallback: Process synchronously if no queue or enqueue failed
	if err := h.service.SyncRepositoriesForInstallation(ctx, installationID); err != nil {
		h.LogError(err, "Failed to sync repositories for installation", "installation_id", installationID)
	}
}

// triggerDeployments triggers deployments for sites matching the repository and branch
func (h *WebhookHandler) triggerDeployments(ctx context.Context, repository, branch string, webhookData map[string]interface{}, providerType gittypes.GitProviderType) {
	// TODO: Get sites by repository and branch from site repository
	// For now, just log the deployment trigger
	h.LogInfo("Would trigger deployments for repository", "repository", repository, "branch", branch, "provider", providerType.String())

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
