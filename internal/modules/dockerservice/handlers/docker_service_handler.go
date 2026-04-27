package handlers

import (
	"context"
	"strconv"

	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/dockerservice/dto"
	"github.com/kkz6/launch-go/internal/modules/dockerservice/services"
	"github.com/kkz6/launch-go/internal/modules/dockerservice/types"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// DockerServiceHandler exposes lifecycle endpoints for docker services
// keyed by `:kind`. CRUD-style endpoints (list/install) are wired
// directly via framework helpers in routes.go.
type DockerServiceHandler struct {
	service *services.Service
}

// NewDockerServiceHandler constructs a handler from its service.
func NewDockerServiceHandler(svc *services.Service) *DockerServiceHandler {
	return &DockerServiceHandler{service: svc}
}

// Show returns one docker service by kind for a server.
func (h *DockerServiceHandler) Show(c *fiber.Ctx) error {
	teamID, serverID, err := fiberctx.GetTeamAndServerID(c)
	if err != nil {
		return err
	}
	kind, err := fiberctx.MustParseEnum(c, "kind", "Invalid docker service kind", types.ParseKind)
	if err != nil {
		return err
	}
	resp, err := h.service.Get(c.Context(), serverID, teamID, kind)
	if err != nil {
		return err
	}
	if resp.ID == "" {
		return fiberctx.NotFound("Docker service not installed")
	}
	return fiberctx.OK(c, "Docker service retrieved", resp)
}

// Uninstall removes a docker service by kind.
func (h *DockerServiceHandler) Uninstall(c *fiber.Ctx) error {
	teamID, serverID, err := fiberctx.GetTeamAndServerID(c)
	if err != nil {
		return err
	}
	kind, err := fiberctx.MustParseEnum(c, "kind", "Invalid docker service kind", types.ParseKind)
	if err != nil {
		return err
	}

	var req dto.UninstallDockerServiceRequest
	if len(c.Body()) > 0 {
		if err := c.BodyParser(&req); err != nil {
			return fiberctx.BadRequest("Invalid request body")
		}
	}
	if c.Query("remove_data") == "true" {
		req.RemoveData = true
	}

	userID, _ := fiberctx.GetUserID(c)
	if err := h.service.Uninstall(c.Context(), serverID, teamID, userID, kind, req.RemoveData); err != nil {
		return err
	}
	return fiberctx.OK(c, "Docker service will be uninstalled shortly", nil)
}

// Start runs `docker start` on the docker service container.
func (h *DockerServiceHandler) Start(c *fiber.Ctx) error {
	return h.lifecycle(c, h.service.Start, "Docker service started")
}

// Stop runs `docker stop` on the docker service container.
func (h *DockerServiceHandler) Stop(c *fiber.Ctx) error {
	return h.lifecycle(c, h.service.Stop, "Docker service stopped")
}

// Restart runs `docker restart` on the docker service container.
func (h *DockerServiceHandler) Restart(c *fiber.Ctx) error {
	return h.lifecycle(c, h.service.Restart, "Docker service restarted")
}

// Logs returns the most recent log lines for the docker service.
func (h *DockerServiceHandler) Logs(c *fiber.Ctx) error {
	teamID, serverID, err := fiberctx.GetTeamAndServerID(c)
	if err != nil {
		return err
	}
	kind, err := fiberctx.MustParseEnum(c, "kind", "Invalid docker service kind", types.ParseKind)
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
	return fiberctx.OK(c, "Docker service logs retrieved", resp)
}

func (h *DockerServiceHandler) lifecycle(
	c *fiber.Ctx,
	fn func(ctx context.Context, serverID, teamID, userID string, kind types.Kind) error,
	msg string,
) error {
	teamID, serverID, err := fiberctx.GetTeamAndServerID(c)
	if err != nil {
		return err
	}
	kind, err := fiberctx.MustParseEnum(c, "kind", "Invalid docker service kind", types.ParseKind)
	if err != nil {
		return err
	}
	userID, _ := fiberctx.GetUserID(c)
	if err := fn(c.Context(), serverID, teamID, userID, kind); err != nil {
		return err
	}
	return fiberctx.OK(c, msg, nil)
}
