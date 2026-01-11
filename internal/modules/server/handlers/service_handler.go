package handlers

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/server/dto"
	"github.com/kkz6/launch-go/internal/modules/server/enums"
	"github.com/kkz6/launch-go/internal/modules/server/services"
	"github.com/kkz6/launch-go/internal/pkg/response"
	"github.com/kkz6/launch-go/internal/pkg/validator"
)

// ListServices returns all installed services for a server
func (h *Handler) ListServices(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	serverID := c.Params("id")

	svcs, err := h.service.ListServices(c.Context(), serverID, teamID)
	if err != nil {
		if errors.Is(err, services.ErrServerNotFound) {
			return response.NotFound(c, "Server not found")
		}

		return response.InternalError(c, "Failed to fetch services")
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

	if errors := validator.Validate(&req); errors != nil {
		return response.ValidationError(c, errors)
	}

	svc, err := h.service.InstallService(c.Context(), serverID, teamID, &req)
	if err != nil {
		if errors.Is(err, services.ErrServerNotFound) {
			return response.NotFound(c, "Server not found")
		}

		if errors.Is(err, services.ErrInvalidSoftware) {
			return response.Error(c, fiber.StatusBadRequest, "Invalid software")
		}

		if errors.Is(err, services.ErrServiceAlreadyExists) {
			return response.Error(c, fiber.StatusConflict, "Service already installed")
		}

		return response.Error(c, fiber.StatusBadRequest, err.Error())
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

	if errors := validator.Validate(&req); errors != nil {
		return response.ValidationError(c, errors)
	}

	operation, err := enums.ParseServiceOption(req.Operation)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid operation")
	}

	if err := h.service.HandleServiceOperation(c.Context(), serverID, teamID, serviceID, operation); err != nil {
		if errors.Is(err, services.ErrServerNotFound) {
			return response.NotFound(c, "Server not found")
		}

		if errors.Is(err, services.ErrServiceNotFound) {
			return response.NotFound(c, "Service not found")
		}

		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "Service operation initiated", nil)
}
