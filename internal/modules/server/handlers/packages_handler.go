package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/server/dto"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// UpdateComposerAuth updates the Composer auth.json configuration on the
// server. PUT to a parent-scoped resource with no own id, so it does not
// fit the generic Update / UpdateNested helpers.
func (h *Handler) UpdateComposerAuth(c *fiber.Ctx, req *dto.UpdateComposerAuthRequest) error {
	teamID, serverID, err := fiberctx.GetTeamAndServerID(c)
	if err != nil {
		return err
	}
	if err := h.service.UpdateComposerAuth(c.Context(), serverID, teamID, req); err != nil {
		return err
	}
	return fiberctx.OK(c, "Composer auth updated", nil)
}
