package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/docker/models"
	"github.com/kkz6/launch-go/internal/modules/docker/repositories"
	"github.com/kkz6/launch-go/internal/modules/docker/types"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// DockerVolumeHandler handles HTTP requests for docker service volumes
type DockerVolumeHandler struct {
	volumeRepo  *repositories.DockerVolumeRepository
	serviceRepo *repositories.DockerServiceRepository
}

// NewDockerVolumeHandler creates a new DockerVolumeHandler
func NewDockerVolumeHandler(volumeRepo *repositories.DockerVolumeRepository, serviceRepo *repositories.DockerServiceRepository) *DockerVolumeHandler {
	return &DockerVolumeHandler{
		volumeRepo:  volumeRepo,
		serviceRepo: serviceRepo,
	}
}

// List retrieves all volumes for a docker service
func (h *DockerVolumeHandler) List(c *fiber.Ctx) error {
	serviceID := c.Params("id")

	if _, err := h.serviceRepo.FindByID(c.Context(), serviceID); err != nil {
		if fiberutil.IsNotFound(err) {
			return fiberutil.RespondNotFound(c, "Docker service not found")
		}

		return fiberutil.RespondInternalError(c, fiberutil.MsgInternalError)
	}

	volumes, err := h.volumeRepo.FindByServiceID(c.Context(), serviceID)
	if err != nil {
		return fiberutil.RespondInternalError(c, fiberutil.MsgInternalError)
	}

	return fiberutil.OK(c, "Volumes retrieved", volumes)
}

// Create creates a new volume for a docker service
func (h *DockerVolumeHandler) Create(c *fiber.Ctx) error {
	serviceID := c.Params("id")

	if _, err := h.serviceRepo.FindByID(c.Context(), serviceID); err != nil {
		if fiberutil.IsNotFound(err) {
			return fiberutil.RespondNotFound(c, "Docker service not found")
		}

		return fiberutil.RespondInternalError(c, fiberutil.MsgInternalError)
	}

	var req struct {
		MountType types.MountType `json:"mount_type" validate:"required"`
		Source    string          `json:"source" validate:"required"`
		Target    string          `json:"target" validate:"required"`
		ReadOnly  bool            `json:"read_only"`
	}

	if err := fiberutil.ParseAndValidate(c, &req); err != nil {
		return err
	}

	volume := &models.DockerVolume{
		DockerServiceID: serviceID,
		MountType:       req.MountType,
		Source:          req.Source,
		Target:          req.Target,
		ReadOnly:        req.ReadOnly,
	}

	if err := h.volumeRepo.Create(c.Context(), volume); err != nil {
		return fiberutil.RespondInternalError(c, fiberutil.MsgInternalError)
	}

	return fiberutil.Created(c, "Volume created", volume)
}

// Delete removes a volume from a docker service
func (h *DockerVolumeHandler) Delete(c *fiber.Ctx) error {
	volumeID := c.Params("volumeId")

	if err := h.volumeRepo.Delete(c.Context(), volumeID); err != nil {
		if fiberutil.IsNotFound(err) {
			return fiberutil.RespondNotFound(c, "Volume not found")
		}

		return fiberutil.RespondInternalError(c, fiberutil.MsgInternalError)
	}

	return fiberutil.OK(c, "Volume deleted", nil)
}
