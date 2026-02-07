package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/docker/models"
	"github.com/kkz6/launch-go/internal/modules/docker/repositories"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
)

type envVarEntry struct {
	Key      string `json:"key" validate:"required"`
	Value    string `json:"value" validate:"required"`
	IsSecret bool   `json:"is_secret"`
}

// DockerEnvHandler handles HTTP requests for docker service environment variables
type DockerEnvHandler struct {
	envRepo     *repositories.DockerEnvVarRepository
	serviceRepo *repositories.DockerServiceRepository
}

// NewDockerEnvHandler creates a new DockerEnvHandler
func NewDockerEnvHandler(envRepo *repositories.DockerEnvVarRepository, serviceRepo *repositories.DockerServiceRepository) *DockerEnvHandler {
	return &DockerEnvHandler{
		envRepo:     envRepo,
		serviceRepo: serviceRepo,
	}
}

// List retrieves all env vars for a docker service with secrets masked
func (h *DockerEnvHandler) List(c *fiber.Ctx) error {
	serviceID := c.Params("id")

	if _, err := h.serviceRepo.FindByID(c.Context(), serviceID); err != nil {
		if fiberutil.IsNotFound(err) {
			return fiberutil.RespondNotFound(c, "Docker service not found")
		}

		return fiberutil.RespondInternalError(c, fiberutil.MsgInternalError)
	}

	envVars, err := h.envRepo.FindByServiceID(c.Context(), serviceID)
	if err != nil {
		return fiberutil.RespondInternalError(c, fiberutil.MsgInternalError)
	}

	for i := range envVars {
		if envVars[i].IsSecret {
			envVars[i].Value = "********"
		}
	}

	return fiberutil.OK(c, "Environment variables retrieved", envVars)
}

// BulkReplace replaces all env vars for a docker service
func (h *DockerEnvHandler) BulkReplace(c *fiber.Ctx) error {
	serviceID := c.Params("id")

	if _, err := h.serviceRepo.FindByID(c.Context(), serviceID); err != nil {
		if fiberutil.IsNotFound(err) {
			return fiberutil.RespondNotFound(c, "Docker service not found")
		}

		return fiberutil.RespondInternalError(c, fiberutil.MsgInternalError)
	}

	var req struct {
		EnvVars []envVarEntry `json:"env_vars" validate:"required,dive"`
	}

	if err := fiberutil.ParseAndValidate(c, &req); err != nil {
		return err
	}

	envVars := make([]models.DockerEnvVar, len(req.EnvVars))
	for i, ev := range req.EnvVars {
		envVars[i] = models.DockerEnvVar{
			DockerServiceID: serviceID,
			Key:             ev.Key,
			Value:           ev.Value,
			IsSecret:        ev.IsSecret,
		}
	}

	result, err := h.envRepo.BulkReplace(c.Context(), serviceID, envVars)
	if err != nil {
		return fiberutil.RespondInternalError(c, fiberutil.MsgInternalError)
	}

	for i := range result {
		if result[i].IsSecret {
			result[i].Value = "********"
		}
	}

	return fiberutil.OK(c, "Environment variables updated", result)
}
