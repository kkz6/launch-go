package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/docker/services"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// DockerComposeHandler handles HTTP requests for docker compose imports
type DockerComposeHandler struct {
	service *services.DockerService
}

// NewDockerComposeHandler creates a new DockerComposeHandler
func NewDockerComposeHandler(svc *services.DockerService) *DockerComposeHandler {
	return &DockerComposeHandler{service: svc}
}

// Preview parses a compose file and returns the parsed services without creating anything
func (h *DockerComposeHandler) Preview(c *fiber.Ctx) error {
	var req struct {
		Content string `json:"content" validate:"required"`
	}

	if err := fiberutil.ParseAndValidate(c, &req); err != nil {
		return err
	}

	parsed, err := services.ParseComposeFile(req.Content)
	if err != nil {
		return fiberutil.RespondBadRequest(c, err.Error())
	}

	return fiberutil.OK(c, "Compose file parsed", parsed)
}

// Import parses a compose file and creates all services
func (h *DockerComposeHandler) Import(c *fiber.Ctx) error {
	serverID := c.Params("serverId")

	teamID, userID, err := fiberutil.MustGetTeamAndUserID(c)
	if err != nil {
		return err
	}

	var req struct {
		Content     string `json:"content" validate:"required"`
		ProjectName string `json:"project_name" validate:"required"`
	}

	if err := fiberutil.ParseAndValidate(c, &req); err != nil {
		return err
	}

	result, err := h.service.ImportCompose(c.Context(), serverID, teamID, userID, req.ProjectName, req.Content)
	if err != nil {
		return fiberutil.RespondBadRequest(c, err.Error())
	}

	return fiberutil.Created(c, "Compose project imported", result)
}
