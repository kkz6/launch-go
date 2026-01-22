package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/server/dto"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// ListDaemons returns all daemons for a server
func (h *Handler) ListDaemons(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	serverID, err := fiberctx.GetID(c)
	if err != nil {
		return err
	}

	daemons, err := h.service.ListDaemons(c.Context(), serverID, teamID)
	if err != nil {
		return fiberctx.HandleErrorOrInternal(c, err, "Failed to fetch daemons")
	}

	result := make([]dto.DaemonResponse, len(daemons))
	for i, daemon := range daemons {
		result[i] = dto.ToDaemonResponse(&daemon)
	}

	return fiberctx.OK(c, "Daemons retrieved", result)
}

// CreateDaemon creates a new daemon
func (h *Handler) CreateDaemon(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	serverID, err := fiberctx.GetID(c)
	if err != nil {
		return err
	}

	req, err := fiberctx.MustParseAndValidate[dto.CreateDaemonRequest](c)
	if err != nil {
		return err
	}

	daemon, err := h.service.CreateDaemon(c.Context(), serverID, teamID, req)
	if err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.Created(c, "Daemon created", dto.ToDaemonResponse(daemon))
}

// UpdateDaemon updates a daemon
func (h *Handler) UpdateDaemon(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	serverID, err := fiberctx.GetID(c)
	if err != nil {
		return err
	}

	daemonID, err := fiberctx.GetDaemonID(c)
	if err != nil {
		return err
	}

	req, err := fiberctx.MustParseAndValidate[dto.UpdateDaemonRequest](c)
	if err != nil {
		return err
	}

	daemon, err := h.service.UpdateDaemon(c.Context(), serverID, teamID, daemonID, req)
	if err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.OK(c, "Daemon updated", dto.ToDaemonResponse(daemon))
}

// DeleteDaemon deletes a daemon
func (h *Handler) DeleteDaemon(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	serverID, err := fiberctx.GetID(c)
	if err != nil {
		return err
	}

	daemonID, err := fiberctx.GetDaemonID(c)
	if err != nil {
		return err
	}

	if err := h.service.DeleteDaemon(c.Context(), serverID, teamID, daemonID); err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.NoContent(c)
}

// RestartDaemon restarts a daemon
func (h *Handler) RestartDaemon(c *fiber.Ctx) error {
	teamID, userID, err := fiberctx.MustGetTeamAndUserID(c)
	if err != nil {
		return err
	}

	serverID, err := fiberctx.GetID(c)
	if err != nil {
		return err
	}

	daemonID, err := fiberctx.GetDaemonID(c)
	if err != nil {
		return err
	}

	if err := h.service.RestartDaemon(c.Context(), serverID, teamID, daemonID, &userID); err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.OK(c, "Daemon restart initiated", nil)
}

// SyncDaemons triggers a status synchronization for all daemons
func (h *Handler) SyncDaemons(c *fiber.Ctx) error {
	teamID, userID, err := fiberctx.MustGetTeamAndUserID(c)
	if err != nil {
		return err
	}

	serverID, err := fiberctx.GetID(c)
	if err != nil {
		return err
	}

	if err := h.service.SyncDaemonsStatus(c.Context(), serverID, teamID, &userID); err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.OK(c, "Daemon sync initiated", nil)
}
