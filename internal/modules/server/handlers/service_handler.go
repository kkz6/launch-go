package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/server/dto"
	"github.com/kkz6/launch-go/internal/modules/server/types"
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

	result := make([]dto.ServiceResponse, len(svcs))
	for i, svc := range svcs {
		result[i] = dto.ToServiceResponse(&svc)
	}

	return fiberctx.OK(c, "Services retrieved", result)
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
