package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/server/dto"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/response"
)

// ListDatabases returns all databases for a server
func (h *Handler) ListDatabases(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	serverID := c.Params("id")

	databases, err := h.service.ListDatabases(c.Context(), serverID, teamID)
	if err != nil {
		return response.HandleErrorOrInternalErr(c, err, "Failed to fetch databases")
	}

	return response.OK(c, "Databases retrieved", databases)
}

// ListDatabaseUsers returns all database users for a server
func (h *Handler) ListDatabaseUsers(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	serverID := c.Params("id")

	users, err := h.service.ListDatabaseUsers(c.Context(), serverID, teamID)
	if err != nil {
		return response.HandleErrorOrInternalErr(c, err, "Failed to fetch database users")
	}

	return response.OK(c, "Database users retrieved", users)
}

// CreateDatabase creates a new database
func (h *Handler) CreateDatabase(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	serverID := c.Params("id")

	req, err := fiberctx.MustParseAndValidate[dto.CreateDatabaseRequest](c)
	if err != nil {
		return err
	}

	db, err := h.service.CreateDatabase(c.Context(), serverID, teamID, req)
	if err != nil {
		return response.HandleError(c, err)
	}

	return response.Created(c, "Database created", db)
}

// SyncDatabases syncs databases from the server
func (h *Handler) SyncDatabases(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	serverID := c.Params("id")

	var userID *string
	if uid, uidErr := fiberctx.GetUserID(c); uidErr == nil && uid != "" {
		userID = &uid
	}

	if err := h.service.SyncDatabases(c.Context(), serverID, teamID, userID); err != nil {
		return response.HandleErrorOrInternalErr(c, err, "Failed to sync databases")
	}

	return response.OK(c, "Database sync started", nil)
}
