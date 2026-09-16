package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/server/dto"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// GetPHPConfiguration reads a version-scoped PHP configuration file.
func (h *Handler) GetPHPConfiguration(c *fiber.Ctx) error {
	teamID, serverID, phpID, err := fiberctx.GetTeamServerAndEntityID(c, "phpId")
	if err != nil {
		return err
	}

	configuration, err := h.service.GetPHPConfiguration(c.Context(), serverID, teamID, phpID, c.Params("kind"))
	if err != nil {
		return err
	}
	return fiberctx.OK(c, "PHP configuration retrieved", configuration)
}

// UpdatePHPConfiguration validates and writes a version-scoped PHP
// configuration file.
func (h *Handler) UpdatePHPConfiguration(c *fiber.Ctx, req *dto.UpdatePHPConfigurationRequest) error {
	teamID, serverID, phpID, err := fiberctx.GetTeamServerAndEntityID(c, "phpId")
	if err != nil {
		return err
	}

	configuration, err := h.service.UpdatePHPConfiguration(c.Context(), serverID, teamID, phpID, c.Params("kind"), req)
	if err != nil {
		return err
	}
	return fiberctx.OK(c, "PHP configuration updated", configuration)
}
