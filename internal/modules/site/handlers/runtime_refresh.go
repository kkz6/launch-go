package handlers

import (
	"github.com/gofiber/fiber/v2"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
)

func (h *DeploymentHandler) RefreshRuntime(c *fiber.Ctx) error {
	serverID, siteID, err := fiberctx.GetServerAndSiteID(c)
	if err != nil {
		return err
	}
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}
	userID, err := fiberctx.MustGetUserID(c)
	if err != nil {
		return err
	}
	if err := h.deploymentService.RefreshRuntime(c.Context(), siteID, serverID, teamID, userID); err != nil {
		return err
	}
	return fiberctx.OK(c, "PHP runtime refresh queued", fiber.Map{"status": "queued"})
}
