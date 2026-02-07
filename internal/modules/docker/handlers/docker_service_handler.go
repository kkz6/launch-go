package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/docker/services"
	"github.com/kkz6/launch-go/internal/modules/docker/types"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// DockerServiceHandler handles HTTP requests for docker services
type DockerServiceHandler struct {
	service *services.DockerService
}

// NewDockerServiceHandler creates a new DockerServiceHandler
func NewDockerServiceHandler(svc *services.DockerService) *DockerServiceHandler {
	return &DockerServiceHandler{service: svc}
}

// List retrieves all docker services for a server
func (h *DockerServiceHandler) List(c *fiber.Ctx) error {
	serverID := c.Params("serverId")

	svcs, err := h.service.ListServices(c.Context(), serverID)
	if err != nil {
		return fiberutil.RespondInternalError(c, fiberutil.MsgInternalError)
	}

	return fiberutil.OK(c, "Docker services retrieved", svcs)
}

// Create creates a new docker service
func (h *DockerServiceHandler) Create(c *fiber.Ctx) error {
	serverID := c.Params("serverId")

	teamID, userID, err := fiberutil.MustGetTeamAndUserID(c)
	if err != nil {
		return err
	}

	var req struct {
		Name  string            `json:"name" validate:"required"`
		Image string            `json:"image" validate:"required"`
		Kind  types.ServiceKind `json:"kind" validate:"required"`
	}

	if err := fiberutil.ParseAndValidate(c, &req); err != nil {
		return err
	}

	svc, err := h.service.CreateService(c.Context(), serverID, teamID, userID, req.Name, req.Image, req.Kind)
	if err != nil {
		return fiberutil.RespondInternalError(c, fiberutil.MsgInternalError)
	}

	return fiberutil.Created(c, "Docker service created", svc)
}

// Show retrieves a single docker service by ID
func (h *DockerServiceHandler) Show(c *fiber.Ctx) error {
	id := c.Params("id")

	svc, err := h.service.GetService(c.Context(), id)
	if err != nil {
		if fiberutil.IsNotFound(err) {
			return fiberutil.RespondNotFound(c, "Docker service not found")
		}

		return fiberutil.RespondInternalError(c, fiberutil.MsgInternalError)
	}

	return fiberutil.OK(c, "Docker service retrieved", svc)
}

// Update modifies an existing docker service
func (h *DockerServiceHandler) Update(c *fiber.Ctx) error {
	id := c.Params("id")

	svc, err := h.service.GetService(c.Context(), id)
	if err != nil {
		if fiberutil.IsNotFound(err) {
			return fiberutil.RespondNotFound(c, "Docker service not found")
		}

		return fiberutil.RespondInternalError(c, fiberutil.MsgInternalError)
	}

	if err := c.BodyParser(svc); err != nil {
		return fiberutil.RespondBadRequest(c, "Invalid request body")
	}

	if err := h.service.UpdateService(c.Context(), svc); err != nil {
		return fiberutil.RespondInternalError(c, fiberutil.MsgInternalError)
	}

	return fiberutil.OK(c, "Docker service updated", svc)
}

// Deploy triggers a deployment of the docker service
func (h *DockerServiceHandler) Deploy(c *fiber.Ctx) error {
	id := c.Params("id")

	teamID, _, err := fiberutil.MustGetTeamAndUserID(c)
	if err != nil {
		return err
	}

	deployment, err := h.service.DeployService(c.Context(), id, teamID)
	if err != nil {
		if fiberutil.IsNotFound(err) {
			return fiberutil.RespondNotFound(c, "Docker service not found")
		}

		return fiberutil.RespondInternalError(c, fiberutil.MsgInternalError)
	}

	return fiberutil.OK(c, "Docker service deployment started", deployment)
}

// Stop stops a running docker service
func (h *DockerServiceHandler) Stop(c *fiber.Ctx) error {
	id := c.Params("id")

	svc, err := h.service.GetService(c.Context(), id)
	if err != nil {
		if fiberutil.IsNotFound(err) {
			return fiberutil.RespondNotFound(c, "Docker service not found")
		}

		return fiberutil.RespondInternalError(c, fiberutil.MsgInternalError)
	}

	if err := h.service.StopService(c.Context(), svc); err != nil {
		return fiberutil.RespondInternalError(c, fiberutil.MsgInternalError)
	}

	return fiberutil.OK(c, "Docker service stop initiated", svc)
}

// Start starts a stopped docker service
func (h *DockerServiceHandler) Start(c *fiber.Ctx) error {
	id := c.Params("id")

	svc, err := h.service.GetService(c.Context(), id)
	if err != nil {
		if fiberutil.IsNotFound(err) {
			return fiberutil.RespondNotFound(c, "Docker service not found")
		}

		return fiberutil.RespondInternalError(c, fiberutil.MsgInternalError)
	}

	if err := h.service.StartService(c.Context(), svc); err != nil {
		return fiberutil.RespondInternalError(c, fiberutil.MsgInternalError)
	}

	return fiberutil.OK(c, "Docker service start initiated", svc)
}

// Restart restarts a docker service
func (h *DockerServiceHandler) Restart(c *fiber.Ctx) error {
	id := c.Params("id")

	svc, err := h.service.GetService(c.Context(), id)
	if err != nil {
		if fiberutil.IsNotFound(err) {
			return fiberutil.RespondNotFound(c, "Docker service not found")
		}

		return fiberutil.RespondInternalError(c, fiberutil.MsgInternalError)
	}

	if err := h.service.RestartService(c.Context(), svc); err != nil {
		return fiberutil.RespondInternalError(c, fiberutil.MsgInternalError)
	}

	return fiberutil.OK(c, "Docker service restart initiated", svc)
}

// Delete removes a docker service
func (h *DockerServiceHandler) Delete(c *fiber.Ctx) error {
	id := c.Params("id")

	if err := h.service.DeleteService(c.Context(), id); err != nil {
		if fiberutil.IsNotFound(err) {
			return fiberutil.RespondNotFound(c, "Docker service not found")
		}

		return fiberutil.RespondInternalError(c, fiberutil.MsgInternalError)
	}

	return fiberutil.OK(c, "Docker service deleted", nil)
}
