package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/docker/models"
	"github.com/kkz6/launch-go/internal/modules/docker/repositories"
	"github.com/kkz6/launch-go/internal/modules/docker/types"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// DockerPortHandler handles HTTP requests for docker service ports
type DockerPortHandler struct {
	portRepo    *repositories.DockerPortRepository
	serviceRepo *repositories.DockerServiceRepository
}

// NewDockerPortHandler creates a new DockerPortHandler
func NewDockerPortHandler(portRepo *repositories.DockerPortRepository, serviceRepo *repositories.DockerServiceRepository) *DockerPortHandler {
	return &DockerPortHandler{
		portRepo:    portRepo,
		serviceRepo: serviceRepo,
	}
}

// List retrieves all ports for a docker service
func (h *DockerPortHandler) List(c *fiber.Ctx) error {
	serviceID := c.Params("id")

	if _, err := h.serviceRepo.FindByID(c.Context(), serviceID); err != nil {
		if fiberutil.IsNotFound(err) {
			return fiberutil.RespondNotFound(c, "Docker service not found")
		}

		return fiberutil.RespondInternalError(c, fiberutil.MsgInternalError)
	}

	ports, err := h.portRepo.FindByServiceID(c.Context(), serviceID)
	if err != nil {
		return fiberutil.RespondInternalError(c, fiberutil.MsgInternalError)
	}

	return fiberutil.OK(c, "Ports retrieved", ports)
}

// Create creates a new port mapping for a docker service
func (h *DockerPortHandler) Create(c *fiber.Ctx) error {
	serviceID := c.Params("id")

	if _, err := h.serviceRepo.FindByID(c.Context(), serviceID); err != nil {
		if fiberutil.IsNotFound(err) {
			return fiberutil.RespondNotFound(c, "Docker service not found")
		}

		return fiberutil.RespondInternalError(c, fiberutil.MsgInternalError)
	}

	var req struct {
		HostPort      int            `json:"host_port" validate:"required,min=1,max=65535"`
		ContainerPort int            `json:"container_port" validate:"required,min=1,max=65535"`
		Protocol      types.Protocol `json:"protocol" validate:"required"`
	}

	if err := fiberutil.ParseAndValidate(c, &req); err != nil {
		return err
	}

	port := &models.DockerPort{
		DockerServiceID: serviceID,
		HostPort:        req.HostPort,
		ContainerPort:   req.ContainerPort,
		Protocol:        req.Protocol,
	}

	if err := h.portRepo.Create(c.Context(), port); err != nil {
		return fiberutil.RespondInternalError(c, fiberutil.MsgInternalError)
	}

	return fiberutil.Created(c, "Port created", port)
}

// Delete removes a port mapping from a docker service
func (h *DockerPortHandler) Delete(c *fiber.Ctx) error {
	portID := c.Params("portId")

	if err := h.portRepo.Delete(c.Context(), portID); err != nil {
		if fiberutil.IsNotFound(err) {
			return fiberutil.RespondNotFound(c, "Port not found")
		}

		return fiberutil.RespondInternalError(c, fiberutil.MsgInternalError)
	}

	return fiberutil.OK(c, "Port deleted", nil)
}
