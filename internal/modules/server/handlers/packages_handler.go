package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/server/dto"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/response"
)

// GetComposerAuth returns the Composer auth.json configuration
func (h *Handler) GetComposerAuth(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	serverID, err := fiberctx.GetID(c)
	if err != nil {
		return err
	}

	result, err := h.service.GetComposerAuth(c.Context(), serverID, teamID)
	if err != nil {
		return response.HandleErrorOrInternalErr(c, err, "Failed to fetch Composer auth")
	}

	return response.OK(c, "Composer auth retrieved", result)
}

// UpdateComposerAuth updates the Composer auth.json configuration
func (h *Handler) UpdateComposerAuth(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	serverID, err := fiberctx.GetID(c)
	if err != nil {
		return err
	}

	req, err := fiberctx.MustParseAndValidate[dto.UpdateComposerAuthRequest](c)
	if err != nil {
		return err
	}

	if err := h.service.UpdateComposerAuth(c.Context(), serverID, teamID, req); err != nil {
		return response.HandleErrorOrInternalErr(c, err, "Failed to update Composer auth")
	}

	return response.OK(c, "Composer auth updated", nil)
}
