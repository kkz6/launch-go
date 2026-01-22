package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/database/dto"
	"github.com/kkz6/launch-go/internal/modules/database/services"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
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
		return fiberctx.RespondBadRequest(c, fiberctx.MsgMissingRequiredParams)
	}

	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	databases, err := h.service.ListDatabases(c.Context(), serverID, teamID)
	if err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.OK(c, "Databases retrieved", dto.ToDatabaseResponseList(databases))
}

// CreateDatabase creates a new database
func (h *Handler) CreateDatabase(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	if serverID == "" {
		return fiberctx.RespondBadRequest(c, fiberctx.MsgMissingRequiredParams)
	}

	req, err := fiberctx.MustParseAndValidate[dto.CreateDatabaseRequest](c)
	if err != nil {
		return err
	}

	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}
	userID := getUserIDFromContext(c)

	database, err := h.service.CreateDatabase(c.Context(), serverID, teamID, req, userID)
	if err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.Created(c, "Database will be created shortly", dto.ToDatabaseResponse(database))
}

// GetDatabase gets a database by ID
func (h *Handler) GetDatabase(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	id := c.Params("id")

	if serverID == "" || id == "" {
		return fiberctx.RespondBadRequest(c, fiberctx.MsgMissingRequiredParams)
	}

	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	database, err := h.service.GetDatabase(c.Context(), id, serverID, teamID)
	if err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.OK(c, "Database retrieved", dto.ToDatabaseResponse(database))
}

// DeleteDatabase deletes a database
func (h *Handler) DeleteDatabase(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	id := c.Params("id")

	if serverID == "" || id == "" {
		return fiberctx.RespondBadRequest(c, fiberctx.MsgMissingRequiredParams)
	}

	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}
	userID := getUserIDFromContext(c)

	if err := h.service.DeleteDatabase(c.Context(), id, serverID, teamID, userID); err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.OK(c, "Database will be deleted shortly", nil)
}

// SyncDatabases syncs databases from the server
func (h *Handler) SyncDatabases(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	if serverID == "" {
		return fiberctx.RespondBadRequest(c, fiberctx.MsgMissingRequiredParams)
	}

	userID := getUserIDFromContext(c)

	if err := h.service.SyncDatabases(c.Context(), serverID, userID); err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.OK(c, "Database sync started", nil)
}
