package handlers

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/site/dto"
	"github.com/kkz6/launch-go/internal/modules/site/repositories"
	"github.com/kkz6/launch-go/internal/modules/site/services"
	"github.com/kkz6/launch-go/internal/pkg/response"
	"github.com/kkz6/launch-go/internal/pkg/validator"
)

// CommandHandler handles HTTP requests for command execution
type CommandHandler struct {
	commandService *services.CommandService
}

// NewCommandHandler creates a new command handler
func NewCommandHandler(commandService *services.CommandService) *CommandHandler {
	return &CommandHandler{commandService: commandService}
}

// CreateCommand creates and executes a command
func (h *CommandHandler) CreateCommand(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	siteID := c.Params("id")
	userID := c.Locals("userID").(string)

	var req dto.CreateCommandRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if errs := validator.Validate(&req); errs != nil {
		return response.ValidationError(c, errs)
	}

	cmd, err := h.commandService.Create(c.Context(), siteID, serverID, userID, &req)
	if err != nil {
		if errors.Is(err, repositories.ErrSiteNotFound) {
			return response.NotFound(c, "Site not found")
		}

		if errors.Is(err, services.ErrSiteNotInstalled) {
			return response.Error(c, fiber.StatusBadRequest, "Site is not installed")
		}

		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.Created(c, "Command created", dto.ToCommandResponse(cmd))
}

// ListCommands returns all commands for a site
func (h *CommandHandler) ListCommands(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	siteID := c.Params("id")

	commands, err := h.commandService.List(c.Context(), siteID, serverID)
	if err != nil {
		if errors.Is(err, repositories.ErrSiteNotFound) {
			return response.NotFound(c, "Site not found")
		}

		return response.InternalError(c, "Failed to fetch commands")
	}

	result := make([]dto.CommandResponse, len(commands))
	for i, cmd := range commands {
		result[i] = dto.ToCommandResponse(&cmd)
	}

	return response.OK(c, "Commands retrieved", result)
}

// DeleteCommand deletes a command
func (h *CommandHandler) DeleteCommand(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	siteID := c.Params("id")
	commandID := c.Params("commandId")

	err := h.commandService.Delete(c.Context(), siteID, serverID, commandID)
	if err != nil {
		if errors.Is(err, repositories.ErrSiteNotFound) {
			return response.NotFound(c, "Site not found")
		}

		if errors.Is(err, repositories.ErrCommandNotFound) {
			return response.NotFound(c, "Command not found")
		}

		return response.InternalError(c, "Failed to delete command")
	}

	return response.OK(c, "Command deleted", nil)
}
