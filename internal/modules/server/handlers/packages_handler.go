package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/server/dto"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// GetComposerAuth returns the Composer auth.json configuration
func (h *Handler) GetComposerAuth(c *fiber.Ctx) error {
	teamID, serverID, err := fiberctx.GetTeamAndServerID(c)
	if err != nil {
		return err
	}

	result, err := h.service.GetComposerAuth(c.Context(), serverID, teamID)
	if err != nil {
		return fiberctx.HandleErrorOrInternal(c, err, "Failed to fetch Composer auth")
	}

	return fiberctx.OK(c, "Composer auth retrieved", result)
}

// UpdateComposerAuth updates the Composer auth.json configuration
func (h *Handler) UpdateComposerAuth(c *fiber.Ctx) error {
	teamID, serverID, err := fiberctx.GetTeamAndServerID(c)
	if err != nil {
		return err
	}

	req, err := fiberctx.MustParseAndValidate[dto.UpdateComposerAuthRequest](c)
	if err != nil {
		return err
	}

	if err := h.service.UpdateComposerAuth(c.Context(), serverID, teamID, req); err != nil {
		return fiberctx.HandleErrorOrInternal(c, err, "Failed to update Composer auth")
	}

	return fiberctx.OK(c, "Composer auth updated", nil)
}
