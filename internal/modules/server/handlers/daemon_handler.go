package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/server/dto"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/response"
)

// ListDaemons returns all daemons for a server
func (h *Handler) ListDaemons(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	serverID := c.Params("id")

	daemons, err := h.service.ListDaemons(c.Context(), serverID, teamID)
	if err != nil {
		return response.HandleErrorOrInternalErr(c, err, "Failed to fetch daemons")
	}

	result := make([]dto.DaemonResponse, len(daemons))
	for i, daemon := range daemons {
		result[i] = dto.ToDaemonResponse(&daemon)
	}

	return response.OK(c, "Daemons retrieved", result)
}

// CreateDaemon creates a new daemon
func (h *Handler) CreateDaemon(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	serverID := c.Params("id")

	req, err := fiberctx.MustParseAndValidate[dto.CreateDaemonRequest](c)
	if err != nil {
		return err
	}

	daemon, err := h.service.CreateDaemon(c.Context(), serverID, teamID, req)
	if err != nil {
		return response.HandleError(c, err)
	}

	return response.Created(c, "Daemon created", dto.ToDaemonResponse(daemon))
}

// UpdateDaemon updates a daemon
func (h *Handler) UpdateDaemon(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	serverID := c.Params("id")
	daemonID := c.Params("daemonId")

	req, err := fiberctx.MustParseAndValidate[dto.UpdateDaemonRequest](c)
	if err != nil {
		return err
	}

	daemon, err := h.service.UpdateDaemon(c.Context(), serverID, teamID, daemonID, req)
	if err != nil {
		return response.HandleError(c, err)
	}

	return response.OK(c, "Daemon updated", dto.ToDaemonResponse(daemon))
}

// DeleteDaemon deletes a daemon
func (h *Handler) DeleteDaemon(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	serverID := c.Params("id")
	daemonID := c.Params("daemonId")

	if err := h.service.DeleteDaemon(c.Context(), serverID, teamID, daemonID); err != nil {
		return response.HandleError(c, err)
	}

	return response.NoContent(c)
}

// RestartDaemon restarts a daemon
func (h *Handler) RestartDaemon(c *fiber.Ctx) error {
	teamID, userID, err := fiberctx.MustGetTeamAndUserID(c)
	if err != nil {
		return err
	}

	serverID := c.Params("id")
	daemonID := c.Params("daemonId")

	if err := h.service.RestartDaemon(c.Context(), serverID, teamID, daemonID, &userID); err != nil {
		return response.HandleError(c, err)
	}

	return response.OK(c, "Daemon restart initiated", nil)
}

// SyncDaemons triggers a status synchronization for all daemons
func (h *Handler) SyncDaemons(c *fiber.Ctx) error {
	teamID, userID, err := fiberctx.MustGetTeamAndUserID(c)
	if err != nil {
		return err
	}

	serverID := c.Params("id")

	if err := h.service.SyncDaemonsStatus(c.Context(), serverID, teamID, &userID); err != nil {
		return response.HandleError(c, err)
	}

	return response.OK(c, "Daemon sync initiated", nil)
}
