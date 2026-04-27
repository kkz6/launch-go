package handlers

import (
	"context"
	"strconv"

	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/managedservice/dto"
	"github.com/kkz6/launch-go/internal/modules/managedservice/services"
	"github.com/kkz6/launch-go/internal/modules/managedservice/types"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// ManagedServiceHandler exposes lifecycle endpoints for managed services
// keyed by `:kind`. CRUD-style endpoints (list/install) are wired
// directly via framework helpers in routes.go.
type ManagedServiceHandler struct {
	service *services.Service
}

// NewManagedServiceHandler constructs a handler from its service.
func NewManagedServiceHandler(svc *services.Service) *ManagedServiceHandler {
	return &ManagedServiceHandler{service: svc}
}

// Show returns one managed service by kind for a server.
func (h *ManagedServiceHandler) Show(c *fiber.Ctx) error {
	teamID, serverID, err := fiberctx.GetTeamAndServerID(c)
	if err != nil {
		return err
	}
	kind, err := fiberctx.MustParseEnum(c, "kind", "Invalid managed service kind", types.ParseKind)
	if err != nil {
		return err
	}
	resp, err := h.service.Get(c.Context(), serverID, teamID, kind)
	if err != nil {
		return err
	}
	if resp.ID == "" {
		return fiberctx.NotFound("Managed service not installed")
	}
	return fiberctx.OK(c, "Managed service retrieved", resp)
}

// Uninstall removes a managed service by kind.
func (h *ManagedServiceHandler) Uninstall(c *fiber.Ctx) error {
	teamID, serverID, err := fiberctx.GetTeamAndServerID(c)
	if err != nil {
		return err
	}
	kind, err := fiberctx.MustParseEnum(c, "kind", "Invalid managed service kind", types.ParseKind)
	if err != nil {
		return err
	}

	var req dto.UninstallManagedServiceRequest
	if len(c.Body()) > 0 {
		if err := c.BodyParser(&req); err != nil {
			return fiberctx.BadRequest("Invalid request body")
		}
	}

	userID, _ := fiberctx.GetUserID(c)
	if err := h.service.Uninstall(c.Context(), serverID, teamID, userID, kind, req.RemoveData); err != nil {
		return err
	}
	return fiberctx.OK(c, "Managed service will be uninstalled shortly", nil)
}

// Start runs `docker start` on the managed service container.
func (h *ManagedServiceHandler) Start(c *fiber.Ctx) error {
	return h.lifecycle(c, h.service.Start, "Managed service started")
}

// Stop runs `docker stop` on the managed service container.
func (h *ManagedServiceHandler) Stop(c *fiber.Ctx) error {
	return h.lifecycle(c, h.service.Stop, "Managed service stopped")
}

// Restart runs `docker restart` on the managed service container.
func (h *ManagedServiceHandler) Restart(c *fiber.Ctx) error {
	return h.lifecycle(c, h.service.Restart, "Managed service restarted")
}

// Logs returns the most recent log lines for the managed service.
func (h *ManagedServiceHandler) Logs(c *fiber.Ctx) error {
	teamID, serverID, err := fiberctx.GetTeamAndServerID(c)
	if err != nil {
		return err
	}
	kind, err := fiberctx.MustParseEnum(c, "kind", "Invalid managed service kind", types.ParseKind)
	if err != nil {
		return err
	}
	tail := 100
	if v := c.Query("tail"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 5000 {
			tail = n
		}
	}
	resp, err := h.service.Logs(c.Context(), serverID, teamID, kind, tail)
	if err != nil {
		return err
	}
	return fiberctx.OK(c, "Managed service logs retrieved", resp)
}

func (h *ManagedServiceHandler) lifecycle(
	c *fiber.Ctx,
	fn func(ctx context.Context, serverID, teamID, userID string, kind types.Kind) error,
	msg string,
) error {
	teamID, serverID, err := fiberctx.GetTeamAndServerID(c)
	if err != nil {
		return err
	}
	kind, err := fiberctx.MustParseEnum(c, "kind", "Invalid managed service kind", types.ParseKind)
	if err != nil {
		return err
	}
	userID, _ := fiberctx.GetUserID(c)
	if err := fn(c.Context(), serverID, teamID, userID, kind); err != nil {
		return err
	}
	return fiberctx.OK(c, msg, nil)
}
