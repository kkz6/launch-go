package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/server/dto"
	"github.com/kkz6/launch-go/internal/pkg/response"
	"github.com/kkz6/launch-go/internal/pkg/validator"
)

// GetComposerAuth returns the Composer auth.json configuration
func (h *Handler) GetComposerAuth(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	serverID := c.Params("id")

	result, err := h.service.GetComposerAuth(c.Context(), serverID, teamID)
	if err != nil {
		return response.HandleErrorOrInternalErr(c, err, "Failed to fetch Composer auth")
	}

	return response.OK(c, "Composer auth retrieved", result)
}

// UpdateComposerAuth updates the Composer auth.json configuration
func (h *Handler) UpdateComposerAuth(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	serverID := c.Params("id")

	var req dto.UpdateComposerAuthRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if errs := validator.Validate(&req); errs != nil {
		return response.ValidationError(c, errs)
	}

	if err := h.service.UpdateComposerAuth(c.Context(), serverID, teamID, &req); err != nil {
		return response.HandleErrorOrInternalErr(c, err, "Failed to update Composer auth")
	}

	return response.OK(c, "Composer auth updated", nil)
}
