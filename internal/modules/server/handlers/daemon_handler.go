package handlers

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/server/dto"
	"github.com/kkz6/launch-go/internal/modules/server/services"
	"github.com/kkz6/launch-go/internal/pkg/response"
	"github.com/kkz6/launch-go/internal/pkg/validator"
)

// ListDaemons returns all daemons for a server
func (h *Handler) ListDaemons(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	serverID := c.Params("id")

	daemons, err := h.service.ListDaemons(c.Context(), serverID, teamID)
	if err != nil {
		if errors.Is(err, services.ErrServerNotFound) {
			return response.NotFound(c, "Server not found")
		}

		return response.InternalError(c, "Failed to fetch daemons")
	}

	result := make([]dto.DaemonResponse, len(daemons))
	for i, daemon := range daemons {
		result[i] = dto.ToDaemonResponse(&daemon)
	}

	return response.OK(c, "Daemons retrieved", result)
}

// CreateDaemon creates a new daemon
func (h *Handler) CreateDaemon(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	serverID := c.Params("id")

	var req dto.CreateDaemonRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if errors := validator.Validate(&req); errors != nil {
		return response.ValidationError(c, errors)
	}

	daemon, err := h.service.CreateDaemon(c.Context(), serverID, teamID, &req)
	if err != nil {
		if errors.Is(err, services.ErrServerNotFound) {
			return response.NotFound(c, "Server not found")
		}

		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.Created(c, "Daemon created", dto.ToDaemonResponse(daemon))
}

// UpdateDaemon updates a daemon
func (h *Handler) UpdateDaemon(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	serverID := c.Params("id")
	daemonID := c.Params("daemonId")

	var req dto.UpdateDaemonRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if errors := validator.Validate(&req); errors != nil {
		return response.ValidationError(c, errors)
	}

	daemon, err := h.service.UpdateDaemon(c.Context(), serverID, teamID, daemonID, &req)
	if err != nil {
		if errors.Is(err, services.ErrServerNotFound) {
			return response.NotFound(c, "Server not found")
		}

		if errors.Is(err, services.ErrDaemonNotFound) {
			return response.NotFound(c, "Daemon not found")
		}

		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "Daemon updated", dto.ToDaemonResponse(daemon))
}

// DeleteDaemon deletes a daemon
func (h *Handler) DeleteDaemon(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	serverID := c.Params("id")
	daemonID := c.Params("daemonId")

	if err := h.service.DeleteDaemon(c.Context(), serverID, teamID, daemonID); err != nil {
		if errors.Is(err, services.ErrServerNotFound) {
			return response.NotFound(c, "Server not found")
		}

		if errors.Is(err, services.ErrDaemonNotFound) {
			return response.NotFound(c, "Daemon not found")
		}

		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.NoContent(c)
}
