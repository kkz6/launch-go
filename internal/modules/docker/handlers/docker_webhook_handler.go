package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/docker/repositories"
	"github.com/kkz6/launch-go/internal/modules/docker/services"
)

// DockerWebhookHandler handles webhook requests for docker deployments
type DockerWebhookHandler struct {
	service     *services.DockerService
	serviceRepo *repositories.DockerServiceRepository
}

// NewDockerWebhookHandler creates a new DockerWebhookHandler
func NewDockerWebhookHandler(svc *services.DockerService, serviceRepo *repositories.DockerServiceRepository) *DockerWebhookHandler {
	return &DockerWebhookHandler{
		service:     svc,
		serviceRepo: serviceRepo,
	}
}

// Deploy handles webhook-triggered deployments
// POST /webhooks/docker/:serviceId/:token
func (h *DockerWebhookHandler) Deploy(c *fiber.Ctx) error {
	serviceID := c.Params("serviceId")
	token := c.Params("token")

	svc, err := h.serviceRepo.FindByID(c.Context(), serviceID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Service not found"})
	}

	// Validate deploy token
	if svc.DeployToken != token {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid token"})
	}

	// Optionally accept a new image
	var req struct {
		Image string `json:"image"`
	}

	if err := c.BodyParser(&req); err == nil && req.Image != "" {
		svc.Image = req.Image
		if err := h.serviceRepo.Update(c.Context(), svc); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to update image"})
		}
	}

	deployment, err := h.service.DeployServiceViaWebhook(c.Context(), svc)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to start deployment"})
	}

	return c.JSON(fiber.Map{
		"success":       true,
		"message":       "Deployment started",
		"deployment_id": deployment.ID,
	})
}
