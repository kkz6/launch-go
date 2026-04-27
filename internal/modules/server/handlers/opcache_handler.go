package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/server/dto"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// GetOpcacheDefaults returns the static default OPcache settings.
// Not team-scoped — does not fit Index.
func (h *Handler) GetOpcacheDefaults(c *fiber.Ctx) error {
	return fiberctx.OK(c, "OPcache defaults retrieved", dto.GetDefaultOpcacheSettings())
}

// ConfigureOpcache configures OPcache settings for a PHP version.
// Action with body — does not fit ActionItemNested.
func (h *Handler) ConfigureOpcache(c *fiber.Ctx) error {
	teamID, serverID, phpID, err := fiberctx.GetTeamServerAndEntityID(c, "phpId")
	if err != nil {
		return err
	}
	req, err := fiberctx.MustParseAndValidate[dto.ConfigureOpcacheRequest](c)
	if err != nil {
		return err
	}
	if err := h.service.ConfigureOpcache(c.Context(), serverID, teamID, phpID, req); err != nil {
		return err
	}
	return fiberctx.OK(c, "OPcache configuration initiated", nil)
}
