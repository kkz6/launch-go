package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/server/dto"
	"github.com/kkz6/launch-go/internal/modules/server/types"
	pkgdto "github.com/kkz6/launch-go/internal/pkg/dto"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// ListServices returns all installed services for a server
func (h *Handler) ListServices(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	serverID, err := fiberctx.GetID(c)
	if err != nil {
		return err
	}

	svcs, err := h.service.ListServices(c.Context(), serverID, teamID)
	if err != nil {
		return fiberctx.HandleErrorOrInternal(c, err, "Failed to fetch services")
	}

	return fiberctx.OK(c, "Services retrieved", pkgdto.TransformSlice(svcs, dto.ToServiceResponse))
}

// InstallService installs a new service on a server
func (h *Handler) InstallService(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	serverID, err := fiberctx.GetID(c)
	if err != nil {
		return err
	}

	req, err := fiberctx.MustParseAndValidate[dto.CreateServiceRequest](c)
	if err != nil {
		return err
	}

	svc, err := h.service.InstallService(c.Context(), serverID, teamID, req)
	if err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.Created(c, "Service installation initiated", dto.ToServiceResponse(svc))
}

// ServiceOperation performs an operation on a service (start, stop, restart)
func (h *Handler) ServiceOperation(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	serverID, err := fiberctx.GetID(c)
	if err != nil {
		return err
	}

	serviceID, err := fiberctx.GetULIDParam(c, "serviceId")
	if err != nil {
		return err
	}

	req, err := fiberctx.MustParseAndValidate[dto.ServiceOperationRequest](c)
	if err != nil {
		return err
	}

	operation, err := types.ParseServiceOption(req.Operation)
	if err != nil {
		return fiberctx.RespondBadRequest(c, "Invalid operation")
	}

	if err := h.service.HandleServiceOperation(c.Context(), serverID, teamID, serviceID, operation); err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.OK(c, "Service operation initiated", nil)
}

// ServiceOperationByAction performs an operation on a service using the action from the URL path
func (h *Handler) ServiceOperationByAction(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	serverID, err := fiberctx.GetID(c)
	if err != nil {
		return err
	}

	serviceID, err := fiberctx.GetULIDParam(c, "serviceId")
	if err != nil {
		return err
	}

	action := c.Params("action")

	operation, err := types.ParseServiceOption(action)
	if err != nil {
		return fiberctx.RespondBadRequest(c, "Invalid operation")
	}

	if err := h.service.HandleServiceOperation(c.Context(), serverID, teamID, serviceID, operation); err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.OK(c, "Service operation initiated", nil)
}

// ListPhpVersions returns all PHP versions with their installation status for a server
func (h *Handler) ListPhpVersions(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	serverID, err := fiberctx.GetID(c)
	if err != nil {
		return err
	}

	phpVersions, err := h.service.GetPhpVersions(c.Context(), serverID, teamID)
	if err != nil {
		return fiberctx.HandleErrorOrInternal(c, err, "Failed to fetch PHP versions")
	}

	return fiberctx.OK(c, "PHP versions retrieved", phpVersions)
}

// ListInstalledPhpVersions returns only the installed PHP versions for a server
func (h *Handler) ListInstalledPhpVersions(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	serverID, err := fiberctx.GetID(c)
	if err != nil {
		return err
	}

	phpVersions, err := h.service.GetInstalledPhpVersions(c.Context(), serverID, teamID)
	if err != nil {
		return fiberctx.HandleErrorOrInternal(c, err, "Failed to fetch installed PHP versions")
	}

	return fiberctx.OK(c, "Installed PHP versions retrieved", phpVersions)
}

// GetAvailableServices returns all available services that can be installed on a server
func (h *Handler) GetAvailableServices(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	serverID, err := fiberctx.GetID(c)
	if err != nil {
		return err
	}

	services, err := h.service.GetAvailableServices(c.Context(), serverID, teamID)
	if err != nil {
		return fiberctx.HandleErrorOrInternal(c, err, "Failed to fetch available services")
	}

	return fiberctx.OK(c, "Available services retrieved", services)
}

// InstallPhpExtension installs a PHP extension on a server
func (h *Handler) InstallPhpExtension(c *fiber.Ctx) error {
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

	req, err := fiberctx.MustParseAndValidate[dto.InstallPhpExtensionRequest](c)
	if err != nil {
		return err
	}

	// Get the PHP service to find its version
	svc, err := h.service.GetServiceStatus(c.Context(), serverID, teamID, phpID)
	if err != nil {
		return fiberctx.HandleError(c, err)
	}

	if svc.Type != types.ServiceTypePhp {
		return fiberctx.RespondBadRequest(c, "Service is not a PHP installation")
	}

	userID, err := fiberctx.GetUserID(c)
	if err != nil {
		return err
	}

	if err := h.service.InstallPhpExtension(c.Context(), serverID, teamID, svc.Version, req.Extension, &userID); err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.OK(c, "Extension installation initiated", nil)
}

// UninstallPhpExtension uninstalls a PHP extension from a server
func (h *Handler) UninstallPhpExtension(c *fiber.Ctx) error {
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

	extension := c.Params("extension")
	if extension == "" {
		return fiberctx.RespondBadRequest(c, "Extension name is required")
	}

	// Get the PHP service to find its version
	svc, err := h.service.GetServiceStatus(c.Context(), serverID, teamID, phpID)
	if err != nil {
		return fiberctx.HandleError(c, err)
	}

	if svc.Type != types.ServiceTypePhp {
		return fiberctx.RespondBadRequest(c, "Service is not a PHP installation")
	}

	userID, err := fiberctx.GetUserID(c)
	if err != nil {
		return err
	}

	if err := h.service.UninstallPhpExtension(c.Context(), serverID, teamID, svc.Version, extension, &userID); err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.OK(c, "Extension uninstall initiated", nil)
}
