package server

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/pkg/response"
	"github.com/kkz6/launch-go/internal/pkg/validator"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) List(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)

	servers, err := h.service.List(c.Context(), teamID)
	if err != nil {
		return response.InternalError(c, "Failed to fetch servers")
	}

	result := make([]ServerResponse, len(servers))
	for i, server := range servers {
		result[i] = ToServerResponse(&server)
	}

	return response.OK(c, "Servers retrieved", result)
}

func (h *Handler) Create(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)

	var req CreateServerRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if errors := validator.Validate(&req); errors != nil {
		return response.ValidationError(c, errors)
	}

	server, err := h.service.Create(c.Context(), teamID, &req)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.Created(c, "Server created", ToServerResponse(server))
}

func (h *Handler) Show(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	id := c.Params("id")

	server, err := h.service.FindByID(c.Context(), id, teamID)
	if err != nil {
		return response.NotFound(c, "Server not found")
	}

	return response.OK(c, "Server retrieved", ToServerResponse(server))
}

func (h *Handler) Update(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	id := c.Params("id")

	var req UpdateServerRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if errors := validator.Validate(&req); errors != nil {
		return response.ValidationError(c, errors)
	}

	server, err := h.service.Update(c.Context(), id, teamID, &req)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "Server updated", ToServerResponse(server))
}

func (h *Handler) Delete(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	id := c.Params("id")

	if err := h.service.Delete(c.Context(), id, teamID); err != nil {
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.NoContent(c)
}

func (h *Handler) Reboot(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	id := c.Params("id")

	if err := h.service.Reboot(c.Context(), id, teamID); err != nil {
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "Server reboot initiated", nil)
}

// Database endpoints
func (h *Handler) ListDatabases(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	serverID := c.Params("id")

	databases, err := h.service.ListDatabases(c.Context(), serverID, teamID)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "Databases retrieved", databases)
}

func (h *Handler) CreateDatabase(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	serverID := c.Params("id")

	var req CreateDatabaseRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if errors := validator.Validate(&req); errors != nil {
		return response.ValidationError(c, errors)
	}

	db, err := h.service.CreateDatabase(c.Context(), serverID, teamID, &req)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.Created(c, "Database created", db)
}
