package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/server/dto"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/response"
)

// GetOpcacheDefaults returns the default OPcache settings
func (h *Handler) GetOpcacheDefaults(c *fiber.Ctx) error {
	return response.OK(c, "OPcache defaults retrieved", dto.GetDefaultOpcacheSettings())
}

// GetOpcacheStatus returns the OPcache status for a PHP version
func (h *Handler) GetOpcacheStatus(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	serverID, err := fiberctx.GetID(c)
	if err != nil {
		return err
	}

	phpID, err := fiberctx.GetULIDParam(c, "phpId")
	if err != nil {
		return err
	}

	status, err := h.service.GetOpcacheStatus(c.Context(), serverID, teamID, phpID)
	if err != nil {
		return response.HandleErrorOrInternalErr(c, err, "Failed to fetch OPcache status")
	}

	return response.OK(c, "OPcache status retrieved", status)
}

// ResetOpcache resets the OPcache for a PHP version
func (h *Handler) ResetOpcache(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	serverID, err := fiberctx.GetID(c)
	if err != nil {
		return err
	}

	phpID, err := fiberctx.GetULIDParam(c, "phpId")
	if err != nil {
		return err
	}

	if err := h.service.ResetOpcache(c.Context(), serverID, teamID, phpID); err != nil {
		return response.HandleErrorOrInternalErr(c, err, "Failed to reset OPcache")
	}

	return response.OK(c, "OPcache reset initiated", nil)
}

// ConfigureOpcache configures OPcache settings for a PHP version
func (h *Handler) ConfigureOpcache(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	serverID, err := fiberctx.GetID(c)
	if err != nil {
		return err
	}

	phpID, err := fiberctx.GetULIDParam(c, "phpId")
	if err != nil {
		return err
	}

	req, err := fiberctx.MustParseAndValidate[dto.ConfigureOpcacheRequest](c)
	if err != nil {
		return err
	}

	if err := h.service.ConfigureOpcache(c.Context(), serverID, teamID, phpID, req); err != nil {
		return response.HandleErrorOrInternalErr(c, err, "Failed to configure OPcache")
	}

	return response.OK(c, "OPcache configuration initiated", nil)
}
