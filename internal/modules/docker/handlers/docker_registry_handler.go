package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/docker/models"
	"github.com/kkz6/launch-go/internal/modules/docker/repositories"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// DockerRegistryHandler handles HTTP requests for docker registries
type DockerRegistryHandler struct {
	repo *repositories.DockerRegistryRepository
}

// NewDockerRegistryHandler creates a new DockerRegistryHandler
func NewDockerRegistryHandler(repo *repositories.DockerRegistryRepository) *DockerRegistryHandler {
	return &DockerRegistryHandler{repo: repo}
}

// List retrieves all docker registries for the current team
func (h *DockerRegistryHandler) List(c *fiber.Ctx) error {
	teamID, err := fiberutil.MustGetTeamID(c)
	if err != nil {
		return err
	}

	registries, err := h.repo.FindByTeamID(c.Context(), teamID)
	if err != nil {
		return fiberutil.RespondInternalError(c, fiberutil.MsgInternalError)
	}

	return fiberutil.OK(c, "Docker registries retrieved", registries)
}

// Create creates a new docker registry
func (h *DockerRegistryHandler) Create(c *fiber.Ctx) error {
	teamID, err := fiberutil.MustGetTeamID(c)
	if err != nil {
		return err
	}

	var req struct {
		Name     string `json:"name" validate:"required"`
		URL      string `json:"url" validate:"required"`
		Username string `json:"username" validate:"required"`
		Password string `json:"password" validate:"required"`
	}

	if err := fiberutil.ParseAndValidate(c, &req); err != nil {
		return err
	}

	registry := &models.DockerRegistry{
		Name:     req.Name,
		URL:      req.URL,
		Username: req.Username,
		Password: dbtype.EncryptedString(req.Password),
	}
	registry.TeamID = teamID

	if err := h.repo.Create(c.Context(), registry); err != nil {
		return fiberutil.RespondInternalError(c, fiberutil.MsgInternalError)
	}

	return fiberutil.Created(c, "Docker registry created", registry)
}

// Show retrieves a single docker registry by ID
func (h *DockerRegistryHandler) Show(c *fiber.Ctx) error {
	id := c.Params("id")

	registry, err := h.repo.FindByID(c.Context(), id)
	if err != nil {
		if fiberutil.IsNotFound(err) {
			return fiberutil.RespondNotFound(c, "Docker registry not found")
		}

		return fiberutil.RespondInternalError(c, fiberutil.MsgInternalError)
	}

	return fiberutil.OK(c, "Docker registry retrieved", registry)
}

// Update modifies an existing docker registry
func (h *DockerRegistryHandler) Update(c *fiber.Ctx) error {
	id := c.Params("id")

	registry, err := h.repo.FindByID(c.Context(), id)
	if err != nil {
		if fiberutil.IsNotFound(err) {
			return fiberutil.RespondNotFound(c, "Docker registry not found")
		}

		return fiberutil.RespondInternalError(c, fiberutil.MsgInternalError)
	}

	var req struct {
		Name     string  `json:"name" validate:"required"`
		URL      string  `json:"url" validate:"required"`
		Username string  `json:"username" validate:"required"`
		Password *string `json:"password"`
	}

	if err := fiberutil.ParseAndValidate(c, &req); err != nil {
		return err
	}

	registry.Name = req.Name
	registry.URL = req.URL
	registry.Username = req.Username

	if req.Password != nil && *req.Password != "" {
		registry.Password = dbtype.EncryptedString(*req.Password)
	}

	if err := h.repo.Update(c.Context(), registry); err != nil {
		return fiberutil.RespondInternalError(c, fiberutil.MsgInternalError)
	}

	return fiberutil.OK(c, "Docker registry updated", registry)
}

// Delete removes a docker registry
func (h *DockerRegistryHandler) Delete(c *fiber.Ctx) error {
	id := c.Params("id")

	if _, err := h.repo.FindByID(c.Context(), id); err != nil {
		if fiberutil.IsNotFound(err) {
			return fiberutil.RespondNotFound(c, "Docker registry not found")
		}

		return fiberutil.RespondInternalError(c, fiberutil.MsgInternalError)
	}

	if err := h.repo.Delete(c.Context(), id); err != nil {
		return fiberutil.RespondInternalError(c, fiberutil.MsgInternalError)
	}

	return fiberutil.OK(c, "Docker registry deleted", nil)
}
