package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/server/dto"
	"github.com/kkz6/launch-go/internal/pkg/response"
	"github.com/kkz6/launch-go/internal/pkg/validator"
)

// GetOpcacheDefaults returns the default OPcache settings
func (h *Handler) GetOpcacheDefaults(c *fiber.Ctx) error {
	return response.OK(c, "OPcache defaults retrieved", dto.GetDefaultOpcacheSettings())
}

// GetOpcacheStatus returns the OPcache status for a PHP version
func (h *Handler) GetOpcacheStatus(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	serverID := c.Params("id")
	phpID := c.Params("phpId")

	status, err := h.service.GetOpcacheStatus(c.Context(), serverID, teamID, phpID)
	if err != nil {
		return response.HandleErrorOrInternalErr(c, err, "Failed to fetch OPcache status")
	}

	return response.OK(c, "OPcache status retrieved", status)
}

// ResetOpcache resets the OPcache for a PHP version
func (h *Handler) ResetOpcache(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	serverID := c.Params("id")
	phpID := c.Params("phpId")

	if err := h.service.ResetOpcache(c.Context(), serverID, teamID, phpID); err != nil {
		return response.HandleErrorOrInternalErr(c, err, "Failed to reset OPcache")
	}

	return response.OK(c, "OPcache reset initiated", nil)
}

// ConfigureOpcache configures OPcache settings for a PHP version
func (h *Handler) ConfigureOpcache(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	serverID := c.Params("id")
	phpID := c.Params("phpId")

	var req dto.ConfigureOpcacheRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if errs := validator.Validate(&req); errs != nil {
		return response.ValidationError(c, errs)
	}

	if err := h.service.ConfigureOpcache(c.Context(), serverID, teamID, phpID, &req); err != nil {
		return response.HandleErrorOrInternalErr(c, err, "Failed to configure OPcache")
	}

	return response.OK(c, "OPcache configuration initiated", nil)
}
