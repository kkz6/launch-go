package handlers

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/site/services"
	apperrors "github.com/kkz6/launch-go/internal/pkg/errors"
	"github.com/kkz6/launch-go/internal/pkg/response"
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
		// Special case: branch mismatch is not an error, just skip deployment
		if errors.Is(err, services.ErrBranchMismatch) {
			return c.SendStatus(fiber.StatusOK)
		}

		// For HTTPStatusError types, use their built-in status code
		var httpErr response.HTTPStatusError
		if errors.As(err, &httpErr) {
			return c.Status(httpErr.HTTPStatus()).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		// Check for site not found
		if errors.Is(err, apperrors.ErrSiteNotFound) {
			return c.SendStatus(fiber.StatusNotFound)
		}

		// Default to internal server error
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Return 204 No Content on success (standard for webhooks)
	return c.SendStatus(fiber.StatusNoContent)
}
