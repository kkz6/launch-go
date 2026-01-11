package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/server/dto"
	"github.com/kkz6/launch-go/internal/modules/server/services"
	"github.com/kkz6/launch-go/internal/pkg/response"
	"github.com/kkz6/launch-go/internal/pkg/validator"
)

// Handler handles all server-related HTTP requests
type Handler struct {
	service *services.Service
}

// NewHandler creates a new server handler
func NewHandler(service *services.Service) *Handler {
	return &Handler{service: service}
}

// List returns all servers for the team
func (h *Handler) List(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)

	servers, err := h.service.ListServers(c.Context(), teamID)
	if err != nil {
		return response.InternalError(c, "Failed to fetch servers")
	}

	result := make([]dto.ServerResponse, len(servers))
	for i, server := range servers {
		result[i] = dto.ToServerResponse(&server)
	}

	return response.OK(c, "Servers retrieved", result)
}

// Create creates a new server
func (h *Handler) Create(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	userID := c.Locals("userID").(string)

	var req dto.CreateServerRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if errs := validator.Validate(&req); errs != nil {
		return response.ValidationError(c, errs)
	}

	server, err := h.service.CreateServer(c.Context(), teamID, userID, &req)
	if err != nil {
		return response.HandleError(c, err)
	}

	return response.Created(c, "Server created", dto.ToServerResponse(server))
}

// Show returns a single server
func (h *Handler) Show(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	id := c.Params("id")

	server, err := h.service.GetServerWithRelations(c.Context(), id, teamID)
	if err != nil {
		return response.HandleErrorOrInternalErr(c, err, "Failed to fetch server")
	}

	return response.OK(c, "Server retrieved", dto.ToServerResponse(server))
}

// Update updates a server
func (h *Handler) Update(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	id := c.Params("id")

	var req dto.UpdateServerRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if errs := validator.Validate(&req); errs != nil {
		return response.ValidationError(c, errs)
	}

	server, err := h.service.UpdateServer(c.Context(), id, teamID, &req)
	if err != nil {
		return response.HandleError(c, err)
	}

	return response.OK(c, "Server updated", dto.ToServerResponse(server))
}

// Delete deletes a server
func (h *Handler) Delete(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	id := c.Params("id")

	if err := h.service.DeleteServer(c.Context(), id, teamID); err != nil {
		return response.HandleError(c, err)
	}

	return response.NoContent(c)
}

// Reboot reboots a server
func (h *Handler) Reboot(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	id := c.Params("id")

	if err := h.service.RebootServer(c.Context(), id, teamID); err != nil {
		return response.HandleError(c, err)
	}

	return response.OK(c, "Server reboot initiated", nil)
}

// Connect tests the connection to a server
func (h *Handler) Connect(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	id := c.Params("id")

	if err := h.service.ConnectServer(c.Context(), id, teamID); err != nil {
		return response.HandleError(c, err)
	}

	return response.OK(c, "Server connection successful", nil)
}

// Archive archives a server
func (h *Handler) Archive(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	id := c.Params("id")

	if err := h.service.ArchiveServer(c.Context(), id, teamID); err != nil {
		return response.HandleError(c, err)
	}

	return response.OK(c, "Server archived", nil)
}

// Unarchive unarchives a server
func (h *Handler) Unarchive(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	id := c.Params("id")

	if err := h.service.UnarchiveServer(c.Context(), id, teamID); err != nil {
		return response.HandleError(c, err)
	}

	return response.OK(c, "Server unarchived", nil)
}

// ShowPage returns aggregated data for the server show page
func (h *Handler) ShowPage(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	id := c.Params("id")

	data, err := h.service.GetShowPageData(c.Context(), id, teamID)
	if err != nil {
		return response.HandleErrorOrInternalErr(c, err, "Failed to fetch server data")
	}

	return response.OK(c, "Server page data retrieved", data)
}
