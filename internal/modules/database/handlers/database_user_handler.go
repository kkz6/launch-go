package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/database/dto"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// ListDatabaseUsers lists all database users for a server
func (h *Handler) ListDatabaseUsers(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	if serverID == "" {
		return fiberctx.RespondBadRequest(c, fiberctx.MsgMissingRequiredParams)
	}

	users, err := h.service.ListDatabaseUsers(c.Context(), serverID)
	if err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.OK(c, "Database users retrieved", dto.ToDatabaseUserResponseList(users))
}

// CreateDatabaseUser creates a new database user
func (h *Handler) CreateDatabaseUser(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	if serverID == "" {
		return fiberctx.RespondBadRequest(c, fiberctx.MsgMissingRequiredParams)
	}

	req, err := fiberctx.MustParseAndValidate[dto.CreateDatabaseUserRequest](c)
	if err != nil {
		return err
	}

	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}
	userID := getUserIDFromContext(c)

	dbUser, err := h.service.CreateDatabaseUser(c.Context(), serverID, teamID, req, userID)
	if err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.Created(c, "Database user will be created shortly", dto.ToDatabaseUserResponse(dbUser))
}

// GetDatabaseUser gets a database user by ID
func (h *Handler) GetDatabaseUser(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	id := c.Params("id")

	if serverID == "" || id == "" {
		return fiberctx.RespondBadRequest(c, fiberctx.MsgMissingRequiredParams)
	}

	user, err := h.service.GetDatabaseUser(c.Context(), id, serverID)
	if err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.OK(c, "Database user retrieved", dto.ToDatabaseUserResponse(user))
}

// UpdateDatabaseUser updates a database user
func (h *Handler) UpdateDatabaseUser(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	id := c.Params("id")

	if serverID == "" || id == "" {
		return fiberctx.RespondBadRequest(c, fiberctx.MsgMissingRequiredParams)
	}

	req, err := fiberctx.MustParseAndValidate[dto.UpdateDatabaseUserRequest](c)
	if err != nil {
		return err
	}

	userID := getUserIDFromContext(c)

	dbUser, err := h.service.UpdateDatabaseUser(c.Context(), id, serverID, req, userID)
	if err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.OK(c, "Database user will be updated shortly", dto.ToDatabaseUserResponse(dbUser))
}

// DeleteDatabaseUser deletes a database user
func (h *Handler) DeleteDatabaseUser(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	id := c.Params("id")

	if serverID == "" || id == "" {
		return fiberctx.RespondBadRequest(c, fiberctx.MsgMissingRequiredParams)
	}

	userID := getUserIDFromContext(c)

	if err := h.service.DeleteDatabaseUser(c.Context(), id, serverID, userID); err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.OK(c, "Database user will be deleted shortly", nil)
}
