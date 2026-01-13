package handlers

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/site/services"
)

// WebhookHandler handles deployment webhook requests
type WebhookHandler struct {
	deploymentService *services.DeploymentService
}

// NewWebhookHandler creates a new webhook handler
func NewWebhookHandler(deploymentService *services.DeploymentService) *WebhookHandler {
	return &WebhookHandler{
		deploymentService: deploymentService,
	}
}

// DeployWebhook handles deployment trigger from git providers (GitHub, GitLab, Bitbucket)
// Route: POST /deploy/:siteId/:token (no auth required)
func (h *WebhookHandler) DeployWebhook(c *fiber.Ctx) error {
	siteID := c.Params("siteId")
	token := c.Params("token")

	if siteID == "" || token == "" {
		return c.SendStatus(fiber.StatusNotFound)
	}

	// Parse webhook payload (commit data from git provider)
	var payload map[string]any
	if err := c.BodyParser(&payload); err != nil {
		// Empty body is OK for simple triggers
		payload = nil
	}

	// Trigger deployment via service
	err := h.deploymentService.DeployFromWebhook(c.Context(), siteID, token, payload)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidDeployToken):
			return c.SendStatus(fiber.StatusForbidden)
		case errors.Is(err, services.ErrBranchMismatch):
			// Not an error, just don't deploy (branch doesn't match)
			return c.SendStatus(fiber.StatusOK)
		case errors.Is(err, services.ErrPendingDeployment):
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{
				"error": "a deployment is already in progress",
			})
		case err.Error() == "site not found":
			return c.SendStatus(fiber.StatusNotFound)
		default:
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": err.Error(),
			})
		}
	}

	// Return 204 No Content on success (standard for webhooks)
	return c.SendStatus(fiber.StatusNoContent)
}
