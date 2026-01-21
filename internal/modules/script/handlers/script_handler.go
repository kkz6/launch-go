package handlers

import (
	"errors"
	"strconv"

	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/script/dto"
	"github.com/kkz6/launch-go/internal/modules/script/repositories"
	"github.com/kkz6/launch-go/internal/modules/script/services"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/response"
)

// ScriptHandler handles HTTP requests for scripts
type ScriptHandler struct {
	service *services.ScriptService
}

// NewScriptHandler creates a new script handler
func NewScriptHandler(service *services.ScriptService) *ScriptHandler {
	return &ScriptHandler{service: service}
}

// List returns all scripts accessible to the user
func (h *ScriptHandler) List(c *fiber.Ctx) error {
	teamID, userID, err := fiberctx.MustGetTeamAndUserID(c)
	if err != nil {
		return err
	}

	scripts, err := h.service.List(c.Context(), userID, teamID)
	if err != nil {
		return response.InternalError(c, response.MsgInternalError)
	}

	results := make([]*dto.ScriptResponse, len(scripts))
	for i, s := range scripts {
		results[i] = dto.ToScriptResponse(&s)
	}

	return response.OK(c, "Scripts retrieved", results)
}

// Show returns a single script
func (h *ScriptHandler) Show(c *fiber.Ctx) error {
	teamID, userID, err := fiberctx.MustGetTeamAndUserID(c)
	if err != nil {
		return err
	}
	scriptID := c.Params("id")

	script, err := h.service.Get(c.Context(), scriptID, userID, teamID)
	if err != nil {
		if errors.Is(err, repositories.ErrScriptNotFound) {
			return response.NotFound(c, "Script not found")
		}

		return response.InternalError(c, response.MsgInternalError)
	}

	return response.OK(c, "Script retrieved", dto.ToScriptResponse(script))
}

// Create creates a new script
func (h *ScriptHandler) Create(c *fiber.Ctx) error {
	userID, err := fiberctx.MustGetUserID(c)
	if err != nil {
		return err
	}

	req, err := fiberctx.MustParseAndValidate[dto.CreateScriptRequest](c)
	if err != nil {
		return err
	}

	script, err := h.service.Create(c.Context(), userID, req)
	if err != nil {
		return response.HandleError(c, err)
	}

	return response.Created(c, "Script created", dto.ToScriptResponse(script))
}

// Update updates a script
func (h *ScriptHandler) Update(c *fiber.Ctx) error {
	teamID, userID, err := fiberctx.MustGetTeamAndUserID(c)
	if err != nil {
		return err
	}
	scriptID := c.Params("id")

	req, err := fiberctx.MustParseAndValidate[dto.UpdateScriptRequest](c)
	if err != nil {
		return err
	}

	script, err := h.service.Update(c.Context(), scriptID, userID, teamID, req)
	if err != nil {
		if errors.Is(err, repositories.ErrScriptNotFound) {
			return response.NotFound(c, "Script not found")
		}

		return response.HandleError(c, err)
	}

	return response.OK(c, "Script updated", dto.ToScriptResponse(script))
}

// Delete deletes a script
func (h *ScriptHandler) Delete(c *fiber.Ctx) error {
	teamID, userID, err := fiberctx.MustGetTeamAndUserID(c)
	if err != nil {
		return err
	}
	scriptID := c.Params("id")

	if err := h.service.Delete(c.Context(), scriptID, userID, teamID); err != nil {
		if errors.Is(err, repositories.ErrScriptNotFound) {
			return response.NotFound(c, "Script not found")
		}

		return response.HandleError(c, err)
	}

	return response.NoContent(c)
}

// Execute executes a script on servers
func (h *ScriptHandler) Execute(c *fiber.Ctx) error {
	teamID, userID, err := fiberctx.MustGetTeamAndUserID(c)
	if err != nil {
		return err
	}
	scriptID := c.Params("id")

	req, err := fiberctx.MustParseAndValidate[dto.ExecuteScriptRequest](c)
	if err != nil {
		return err
	}

	result, err := h.service.Execute(c.Context(), scriptID, userID, teamID, req)
	if err != nil {
		if errors.Is(err, repositories.ErrScriptNotFound) {
			return response.NotFound(c, "Script not found")
		}

		return response.HandleError(c, err)
	}

	return response.OK(c, "Script execution started", result)
}

// ListExecutions returns executions for a script
func (h *ScriptHandler) ListExecutions(c *fiber.Ctx) error {
	teamID, userID, err := fiberctx.MustGetTeamAndUserID(c)
	if err != nil {
		return err
	}
	scriptID := c.Params("id")

	executions, err := h.service.ListExecutions(c.Context(), scriptID, userID, teamID)
	if err != nil {
		if errors.Is(err, repositories.ErrScriptNotFound) {
			return response.NotFound(c, "Script not found")
		}

		return response.InternalError(c, response.MsgInternalError)
	}

	results := make([]*dto.ScriptExecutionResponse, len(executions))
	for i, e := range executions {
		results[i] = dto.ToExecutionResponse(&e)
	}

	return response.OK(c, "Executions retrieved", results)
}

// GetExecution returns a single execution
func (h *ScriptHandler) GetExecution(c *fiber.Ctx) error {
	executionIDStr := c.Params("id")

	executionID, err := strconv.ParseUint(executionIDStr, 10, 64)
	if err != nil {
		return response.BadRequest(c, "Invalid execution ID")
	}

	execution, err := h.service.GetExecution(c.Context(), executionID)
	if err != nil {
		if errors.Is(err, repositories.ErrExecutionNotFound) {
			return response.NotFound(c, "Execution not found")
		}

		return response.InternalError(c, response.MsgInternalError)
	}

	return response.OK(c, "Execution retrieved", dto.ToExecutionResponse(execution))
}
