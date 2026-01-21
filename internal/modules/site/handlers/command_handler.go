package handlers

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/site/dto"
	"github.com/kkz6/launch-go/internal/modules/site/repositories"
	"github.com/kkz6/launch-go/internal/modules/site/services"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/response"
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
	serverID, err := fiberctx.GetServerID(c)
	if err != nil {
		return err
	}

	siteID, err := fiberctx.GetSiteID(c)
	if err != nil {
		return err
	}

	userID, err := fiberctx.MustGetUserID(c)
	if err != nil {
		return err
	}

	req, err := fiberctx.MustParseAndValidate[dto.CreateCommandRequest](c)
	if err != nil {
		return err
	}

	cmd, err := h.commandService.Create(c.Context(), siteID, serverID, userID, req)
	if err != nil {
		if errors.Is(err, repositories.ErrSiteNotFound) {
			return response.NotFound(c, response.MsgSiteNotFound)
		}

		if errors.Is(err, services.ErrSiteNotInstalled) {
			return response.BadRequest(c, response.MsgSiteNotInstalled)
		}

		return response.HandleError(c, err)
	}

	return response.Created(c, "Command created", dto.ToCommandResponse(cmd))
}

// ListCommands returns all commands for a site
func (h *CommandHandler) ListCommands(c *fiber.Ctx) error {
	serverID, err := fiberctx.GetServerID(c)
	if err != nil {
		return err
	}

	siteID, err := fiberctx.GetSiteID(c)
	if err != nil {
		return err
	}

	commands, err := h.commandService.List(c.Context(), siteID, serverID)
	if err != nil {
		if errors.Is(err, repositories.ErrSiteNotFound) {
			return response.NotFound(c, response.MsgSiteNotFound)
		}

		return response.InternalError(c, response.MsgInternalError)
	}

	result := make([]dto.CommandResponse, len(commands))
	for i, cmd := range commands {
		result[i] = dto.ToCommandResponse(&cmd)
	}

	return response.OK(c, "Commands retrieved", result)
}

// DeleteCommand deletes a command
func (h *CommandHandler) DeleteCommand(c *fiber.Ctx) error {
	serverID, err := fiberctx.GetServerID(c)
	if err != nil {
		return err
	}

	siteID, err := fiberctx.GetSiteID(c)
	if err != nil {
		return err
	}

	commandID, err := fiberctx.GetULIDParam(c, "commandId")
	if err != nil {
		return err
	}

	err = h.commandService.Delete(c.Context(), siteID, serverID, commandID)
	if err != nil {
		if errors.Is(err, repositories.ErrSiteNotFound) {
			return response.NotFound(c, response.MsgSiteNotFound)
		}

		if errors.Is(err, repositories.ErrCommandNotFound) {
			return response.NotFound(c, response.MsgResourceNotFound)
		}

		return response.InternalError(c, response.MsgInternalError)
	}

	return response.OK(c, "Command deleted", nil)
}
