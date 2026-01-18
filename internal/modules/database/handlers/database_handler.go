package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/database/dto"
	"github.com/kkz6/launch-go/internal/modules/database/services"
	"github.com/kkz6/launch-go/internal/pkg/response"
	"github.com/kkz6/launch-go/internal/pkg/validator"
)

// Handler handles HTTP requests for database operations
type Handler struct {
	service *services.Service
}

// NewHandler creates a new database handler
func NewHandler(service *services.Service) *Handler {
	return &Handler{service: service}
}

// ListDatabases lists all databases for a server
func (h *Handler) ListDatabases(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	if serverID == "" {
		return response.Error(c, fiber.StatusBadRequest, "Server ID is required")
	}

	teamID := getTeamIDFromContext(c)

	databases, err := h.service.ListDatabases(c.Context(), serverID, teamID)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "Databases retrieved", dto.ToDatabaseResponseList(databases))
}

// CreateDatabase creates a new database
func (h *Handler) CreateDatabase(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	if serverID == "" {
		return response.Error(c, fiber.StatusBadRequest, "Server ID is required")
	}

	var req dto.CreateDatabaseRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if errors := validator.Validate(&req); errors != nil {
		return response.ValidationError(c, errors)
	}

	teamID := getTeamIDFromContext(c)
	userID := getUserIDFromContext(c)

	database, err := h.service.CreateDatabase(c.Context(), serverID, teamID, &req, userID)
	if err != nil {
		return handleServiceError(c, err)
	}

	return response.Created(c, "Database will be created shortly", dto.ToDatabaseResponse(database))
}

// GetDatabase gets a database by ID
func (h *Handler) GetDatabase(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	id := c.Params("id")

	if serverID == "" || id == "" {
		return response.Error(c, fiber.StatusBadRequest, "Server ID and Database ID are required")
	}

	teamID := getTeamIDFromContext(c)

	database, err := h.service.GetDatabase(c.Context(), id, serverID, teamID)
	if err != nil {
		return handleServiceError(c, err)
	}

	return response.OK(c, "Database retrieved", dto.ToDatabaseResponse(database))
}

// DeleteDatabase deletes a database
func (h *Handler) DeleteDatabase(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	id := c.Params("id")

	if serverID == "" || id == "" {
		return response.Error(c, fiber.StatusBadRequest, "Server ID and Database ID are required")
	}

	teamID := getTeamIDFromContext(c)
	userID := getUserIDFromContext(c)

	if err := h.service.DeleteDatabase(c.Context(), id, serverID, teamID, userID); err != nil {
		return handleServiceError(c, err)
	}

	return response.OK(c, "Database will be deleted shortly", nil)
}

// SyncDatabases syncs databases from the server
func (h *Handler) SyncDatabases(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	if serverID == "" {
		return response.Error(c, fiber.StatusBadRequest, "Server ID is required")
	}

	userID := getUserIDFromContext(c)

	if err := h.service.SyncDatabases(c.Context(), serverID, userID); err != nil {
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "Database sync started", nil)
}
