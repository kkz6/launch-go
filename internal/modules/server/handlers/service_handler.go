package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/server/dto"
	"github.com/kkz6/launch-go/internal/modules/server/enums"
	"github.com/kkz6/launch-go/internal/pkg/response"
	"github.com/kkz6/launch-go/internal/pkg/validator"
)

// ListServices returns all installed services for a server
func (h *Handler) ListServices(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	serverID := c.Params("id")

	svcs, err := h.service.ListServices(c.Context(), serverID, teamID)
	if err != nil {
		return response.HandleErrorOrInternalErr(c, err, "Failed to fetch services")
	}

	result := make([]dto.ServiceResponse, len(svcs))
	for i, svc := range svcs {
		result[i] = dto.ToServiceResponse(&svc)
	}

	return response.OK(c, "Services retrieved", result)
}

// InstallService installs a new service on a server
func (h *Handler) InstallService(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	serverID := c.Params("id")

	var req dto.CreateServiceRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if errs := validator.Validate(&req); errs != nil {
		return response.ValidationError(c, errs)
	}

	svc, err := h.service.InstallService(c.Context(), serverID, teamID, &req)
	if err != nil {
		return response.HandleError(c, err)
	}

	return response.Created(c, "Service installation initiated", dto.ToServiceResponse(svc))
}

// ServiceOperation performs an operation on a service (start, stop, restart)
func (h *Handler) ServiceOperation(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	serverID := c.Params("id")
	serviceID := c.Params("serviceId")

	var req dto.ServiceOperationRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if errs := validator.Validate(&req); errs != nil {
		return response.ValidationError(c, errs)
	}

	operation, err := enums.ParseServiceOption(req.Operation)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid operation")
	}

	if err := h.service.HandleServiceOperation(c.Context(), serverID, teamID, serviceID, operation); err != nil {
		return response.HandleError(c, err)
	}

	return response.OK(c, "Service operation initiated", nil)
}

// ListPhpVersions returns all PHP versions with their installation status for a server
func (h *Handler) ListPhpVersions(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	serverID := c.Params("id")

	phpVersions, err := h.service.GetPhpVersions(c.Context(), serverID, teamID)
	if err != nil {
		return response.HandleErrorOrInternalErr(c, err, "Failed to fetch PHP versions")
	}

	return response.OK(c, "PHP versions retrieved", phpVersions)
}

// GetAvailableServices returns all available services that can be installed on a server
func (h *Handler) GetAvailableServices(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	serverID := c.Params("id")

	services, err := h.service.GetAvailableServices(c.Context(), serverID, teamID)
	if err != nil {
		return response.HandleErrorOrInternalErr(c, err, "Failed to fetch available services")
	}

	return response.OK(c, "Available services retrieved", services)
}

// GetServiceStatus returns the current status of a service
func (h *Handler) GetServiceStatus(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	serverID := c.Params("id")
	serviceID := c.Params("serviceId")

	service, err := h.service.GetServiceStatus(c.Context(), serverID, teamID, serviceID)
	if err != nil {
		return response.HandleErrorOrInternalErr(c, err, "Failed to fetch service status")
	}

	return response.OK(c, "Service status retrieved", dto.ToServiceResponse(service))
}

// CheckServiceStatus triggers a status check on the server for a service
func (h *Handler) CheckServiceStatus(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	serverID := c.Params("id")
	serviceID := c.Params("serviceId")

	if err := h.service.CheckServiceStatus(c.Context(), serverID, teamID, serviceID); err != nil {
		return response.HandleError(c, err)
	}

	return response.OK(c, "Service status check initiated", nil)
}
