package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/server/dto"
	"github.com/kkz6/launch-go/internal/modules/server/types"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// ServiceOperation performs an operation on a service (start/stop/restart/...)
// from the request body. Action with body so it does not fit
// ActionItemNested.
func (h *Handler) ServiceOperation(c *fiber.Ctx, req *dto.ServiceOperationRequest) error {
	teamID, serverID, serviceID, err := fiberctx.GetTeamServerAndEntityID(c, "serviceId")
	if err != nil {
		return err
	}
	operation, err := types.ParseServiceOption(req.Operation)
	if err != nil {
		return fiberctx.RespondBadRequest(c, "Invalid operation")
	}
	if err := h.service.HandleServiceOperation(c.Context(), serverID, teamID, serviceID, operation); err != nil {
		return err
	}
	return fiberctx.OK(c, "Service operation initiated", nil)
}

// ServiceOperationByAction performs the operation named in the URL path.
// Three path params (server id, service id, action) — does not fit any
// generic helper.
func (h *Handler) ServiceOperationByAction(c *fiber.Ctx) error {
	teamID, serverID, serviceID, err := fiberctx.GetTeamServerAndEntityID(c, "serviceId")
	if err != nil {
		return err
	}
	operation, err := types.ParseServiceOption(c.Params("action"))
	if err != nil {
		return fiberctx.RespondBadRequest(c, "Invalid operation")
	}
	if err := h.service.HandleServiceOperation(c.Context(), serverID, teamID, serviceID, operation); err != nil {
		return err
	}
	return fiberctx.OK(c, "Service operation initiated", nil)
}

// InstallPhpExtension installs a PHP extension on a server's PHP service.
// Action with body, three implied scopes (server, php, extension) — does
// not fit a generic helper.
func (h *Handler) InstallPhpExtension(c *fiber.Ctx, req *dto.InstallPhpExtensionRequest) error {
	teamID, serverID, phpID, err := fiberctx.GetTeamServerAndEntityID(c, "phpId")
	if err != nil {
		return err
	}

	svc, err := h.service.GetServiceStatus(c.Context(), serverID, teamID, phpID)
	if err != nil {
		return err
	}
	if svc.Type != types.ServiceTypePhp {
		return fiberctx.RespondBadRequest(c, "Service is not a PHP installation")
	}

	userID, err := fiberctx.GetUserID(c)
	if err != nil {
		return err
	}

	if err := h.service.InstallPhpExtension(c.Context(), serverID, teamID, svc.Version, req.Extension, &userID); err != nil {
		return err
	}
	return fiberctx.OK(c, "Extension installation initiated", nil)
}

// UninstallPhpExtension uninstalls a PHP extension. Three path params
// (server id, php id, extension) — does not fit a generic helper.
func (h *Handler) UninstallPhpExtension(c *fiber.Ctx) error {
	teamID, serverID, phpID, err := fiberctx.GetTeamServerAndEntityID(c, "phpId")
	if err != nil {
		return err
	}
	extension := c.Params("extension")
	if extension == "" {
		return fiberctx.RespondBadRequest(c, "Extension name is required")
	}

	svc, err := h.service.GetServiceStatus(c.Context(), serverID, teamID, phpID)
	if err != nil {
		return err
	}
	if svc.Type != types.ServiceTypePhp {
		return fiberctx.RespondBadRequest(c, "Service is not a PHP installation")
	}

	userID, err := fiberctx.GetUserID(c)
	if err != nil {
		return err
	}

	if err := h.service.UninstallPhpExtension(c.Context(), serverID, teamID, svc.Version, extension, &userID); err != nil {
		return err
	}
	return fiberctx.OK(c, "Extension uninstall initiated", nil)
}
