package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/server/dto"
	"github.com/kkz6/launch-go/internal/pkg/response"
)

// GetCreateOptions returns the options for creating a server
func (h *Handler) GetCreateOptions(c *fiber.Ctx) error {
	options := dto.GetCreateServerOptions()
	return response.OK(c, "Create options retrieved", options)
}

// ListServerProviders returns all connected server providers for the team
func (h *Handler) ListServerProviders(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)

	providers, err := h.service.ListServerProviders(c.Context(), teamID)
	if err != nil {
		return response.InternalError(c, "Failed to fetch server providers")
	}

	result := make([]dto.ServerProviderResponse, len(providers))
	for i, provider := range providers {
		result[i] = dto.ToServerProviderResponse(&provider)
	}

	return response.OK(c, "Server providers retrieved", result)
}
