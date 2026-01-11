package database

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/pkg/response"
	"github.com/kkz6/launch-go/internal/pkg/validator"
)

// Handler handles HTTP requests for database operations
type Handler struct {
	service *Service
}

// NewHandler creates a new database handler
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// ListDatabases lists all databases for a server
func (h *Handler) ListDatabases(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	if serverID == "" {
		return response.Error(c, fiber.StatusBadRequest, "Server ID is required")
	}

	databases, err := h.service.ListDatabases(c.Context(), serverID)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "Databases retrieved", ToDatabaseResponseList(databases))
}

// CreateDatabase creates a new database
func (h *Handler) CreateDatabase(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	if serverID == "" {
		return response.Error(c, fiber.StatusBadRequest, "Server ID is required")
	}

	var req CreateDatabaseRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if errors := validator.Validate(&req); errors != nil {
		return response.ValidationError(c, errors)
	}

	userID := getUserIDFromContext(c)

	database, err := h.service.CreateDatabase(c.Context(), serverID, &req, userID)
	if err != nil {
		return handleServiceError(c, err)
	}

	return response.Created(c, "Database will be created shortly", ToDatabaseResponse(database))
}

// GetDatabase gets a database by ID
func (h *Handler) GetDatabase(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	id := c.Params("id")

	if serverID == "" || id == "" {
		return response.Error(c, fiber.StatusBadRequest, "Server ID and Database ID are required")
	}

	database, err := h.service.GetDatabase(c.Context(), id, serverID)
	if err != nil {
		return handleServiceError(c, err)
	}

	return response.OK(c, "Database retrieved", ToDatabaseResponse(database))
}

// DeleteDatabase deletes a database
func (h *Handler) DeleteDatabase(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	id := c.Params("id")

	if serverID == "" || id == "" {
		return response.Error(c, fiber.StatusBadRequest, "Server ID and Database ID are required")
	}

	userID := getUserIDFromContext(c)

	if err := h.service.DeleteDatabase(c.Context(), id, serverID, userID); err != nil {
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

// Database User handlers

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

	return response.OK(c, "Database users retrieved", ToDatabaseUserResponseList(users))
}

// CreateDatabaseUser creates a new database user
func (h *Handler) CreateDatabaseUser(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	if serverID == "" {
		return response.Error(c, fiber.StatusBadRequest, "Server ID is required")
	}

	var req CreateDatabaseUserRequest
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

	return response.Created(c, "Database user will be created shortly", ToDatabaseUserResponse(dbUser))
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

	return response.OK(c, "Database user retrieved", ToDatabaseUserResponse(user))
}

// UpdateDatabaseUser updates a database user
func (h *Handler) UpdateDatabaseUser(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	id := c.Params("id")

	if serverID == "" || id == "" {
		return response.Error(c, fiber.StatusBadRequest, "Server ID and User ID are required")
	}

	var req UpdateDatabaseUserRequest
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

	return response.OK(c, "Database user will be updated shortly", ToDatabaseUserResponse(dbUser))
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

// Helper functions

func getUserIDFromContext(c *fiber.Ctx) *string {
	if userID, ok := c.Locals("userID").(string); ok && userID != "" {
		return &userID
	}

	return nil
}

func handleServiceError(c *fiber.Ctx, err error) error {
	switch err {
	case ErrDatabaseNotFound:
		return response.NotFound(c, "Database not found")
	case ErrDatabaseUserNotFound:
		return response.NotFound(c, "Database user not found")
	case ErrDatabaseNameExists:
		return response.Error(c, fiber.StatusConflict, err.Error())
	case ErrDatabaseUserNameExists:
		return response.Error(c, fiber.StatusConflict, err.Error())
	case ErrDatabaseBeingUninstalled:
		return response.Error(c, fiber.StatusConflict, err.Error())
	case ErrUserBeingUninstalled:
		return response.Error(c, fiber.StatusConflict, err.Error())
	case ErrInvalidExistingUser:
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	default:
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}
}
