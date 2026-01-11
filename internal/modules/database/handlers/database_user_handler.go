package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/database/dto"
	"github.com/kkz6/launch-go/internal/pkg/response"
	"github.com/kkz6/launch-go/internal/pkg/validator"
)

// ListDatabaseUsers lists all database users for a server
func (h *Handler) ListDatabaseUsers(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	if serverID == "" {
		return response.Error(c, fiber.StatusBadRequest, "Server ID is required")
	}

	users, err := h.service.ListDatabaseUsers(c.Context(), serverID)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "Database users retrieved", dto.ToDatabaseUserResponseList(users))
}

// CreateDatabaseUser creates a new database user
func (h *Handler) CreateDatabaseUser(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	if serverID == "" {
		return response.Error(c, fiber.StatusBadRequest, "Server ID is required")
	}

	var req dto.CreateDatabaseUserRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if errors := validator.Validate(&req); errors != nil {
		return response.ValidationError(c, errors)
	}

	userID := getUserIDFromContext(c)

	dbUser, err := h.service.CreateDatabaseUser(c.Context(), serverID, &req, userID)
	if err != nil {
		return handleServiceError(c, err)
	}

	return response.Created(c, "Database user will be created shortly", dto.ToDatabaseUserResponse(dbUser))
}

// GetDatabaseUser gets a database user by ID
func (h *Handler) GetDatabaseUser(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	id := c.Params("id")

	if serverID == "" || id == "" {
		return response.Error(c, fiber.StatusBadRequest, "Server ID and User ID are required")
	}

	user, err := h.service.GetDatabaseUser(c.Context(), id, serverID)
	if err != nil {
		return handleServiceError(c, err)
	}

	return response.OK(c, "Database user retrieved", dto.ToDatabaseUserResponse(user))
}

// UpdateDatabaseUser updates a database user
func (h *Handler) UpdateDatabaseUser(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	id := c.Params("id")

	if serverID == "" || id == "" {
		return response.Error(c, fiber.StatusBadRequest, "Server ID and User ID are required")
	}

	var req dto.UpdateDatabaseUserRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if errors := validator.Validate(&req); errors != nil {
		return response.ValidationError(c, errors)
	}

	userID := getUserIDFromContext(c)

	dbUser, err := h.service.UpdateDatabaseUser(c.Context(), id, serverID, &req, userID)
	if err != nil {
		return handleServiceError(c, err)
	}

	return response.OK(c, "Database user will be updated shortly", dto.ToDatabaseUserResponse(dbUser))
}

// DeleteDatabaseUser deletes a database user
func (h *Handler) DeleteDatabaseUser(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	id := c.Params("id")

	if serverID == "" || id == "" {
		return response.Error(c, fiber.StatusBadRequest, "Server ID and User ID are required")
	}

	userID := getUserIDFromContext(c)

	if err := h.service.DeleteDatabaseUser(c.Context(), id, serverID, userID); err != nil {
		return handleServiceError(c, err)
	}

	return response.OK(c, "Database user will be deleted shortly", nil)
}
