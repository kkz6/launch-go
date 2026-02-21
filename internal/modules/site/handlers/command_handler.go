package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/site/dto"
	"github.com/kkz6/launch-go/internal/modules/site/services"
	pkgdto "github.com/kkz6/launch-go/internal/pkg/dto"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
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
	serverID, siteID, err := fiberctx.GetServerAndSiteID(c)
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
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.Created(c, "Command created", dto.ToCommandResponse(cmd))
}

// ListCommands returns all commands for a site
func (h *CommandHandler) ListCommands(c *fiber.Ctx) error {
	serverID, siteID, err := fiberctx.GetServerAndSiteID(c)
	if err != nil {
		return err
	}

	commands, err := h.commandService.List(c.Context(), siteID, serverID)
	if err != nil {
		return fiberctx.HandleError(c, err)
	}

	result := pkgdto.TransformSlice(commands, dto.ToCommandResponse)

	return fiberctx.OK(c, "Commands retrieved", result)
}

// DeleteCommand deletes a command
func (h *CommandHandler) DeleteCommand(c *fiber.Ctx) error {
	serverID, siteID, commandID, err := fiberctx.GetServerSiteAndEntityID(c, "commandId")
	if err != nil {
		return err
	}

	err = h.commandService.Delete(c.Context(), siteID, serverID, commandID)
	if err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.OK(c, "Command deleted", nil)
}
