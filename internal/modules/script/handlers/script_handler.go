package handlers

import (
	"errors"
	"strconv"

	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/script/dto"
	"github.com/kkz6/launch-go/internal/modules/script/repositories"
	"github.com/kkz6/launch-go/internal/modules/script/services"
	"github.com/kkz6/launch-go/internal/pkg/response"
	"github.com/kkz6/launch-go/internal/pkg/validator"
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
	userID := c.Locals("userID").(string)
	teamID := c.Locals("teamID").(string)

	scripts, err := h.service.List(c.Context(), userID, teamID)
	if err != nil {
		return response.InternalError(c, "Failed to fetch scripts")
	}

	results := make([]*dto.ScriptResponse, len(scripts))
	for i, s := range scripts {
		results[i] = dto.ToScriptResponse(&s)
	}

	return response.OK(c, "Scripts retrieved", results)
}

// Show returns a single script
func (h *ScriptHandler) Show(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)
	teamID := c.Locals("teamID").(string)
	scriptID := c.Params("id")

	script, err := h.service.Get(c.Context(), scriptID, userID, teamID)
	if err != nil {
		if errors.Is(err, repositories.ErrScriptNotFound) {
			return response.NotFound(c, "Script not found")
		}

		return response.InternalError(c, "Failed to fetch script")
	}

	return response.OK(c, "Script retrieved", dto.ToScriptResponse(script))
}

// Create creates a new script
func (h *ScriptHandler) Create(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)

	var req dto.CreateScriptRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if errs := validator.Validate(&req); errs != nil {
		return response.ValidationError(c, errs)
	}

	script, err := h.service.Create(c.Context(), userID, &req)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.Created(c, "Script created", dto.ToScriptResponse(script))
}

// Update updates a script
func (h *ScriptHandler) Update(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)
	teamID := c.Locals("teamID").(string)
	scriptID := c.Params("id")

	var req dto.UpdateScriptRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if errs := validator.Validate(&req); errs != nil {
		return response.ValidationError(c, errs)
	}

	script, err := h.service.Update(c.Context(), scriptID, userID, teamID, &req)
	if err != nil {
		if errors.Is(err, repositories.ErrScriptNotFound) {
			return response.NotFound(c, "Script not found")
		}

		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "Script updated", dto.ToScriptResponse(script))
}

// Delete deletes a script
func (h *ScriptHandler) Delete(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)
	teamID := c.Locals("teamID").(string)
	scriptID := c.Params("id")

	if err := h.service.Delete(c.Context(), scriptID, userID, teamID); err != nil {
		if errors.Is(err, repositories.ErrScriptNotFound) {
			return response.NotFound(c, "Script not found")
		}

		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.NoContent(c)
}

// Execute executes a script on servers
func (h *ScriptHandler) Execute(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)
	teamID := c.Locals("teamID").(string)
	scriptID := c.Params("id")

	var req dto.ExecuteScriptRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if errs := validator.Validate(&req); errs != nil {
		return response.ValidationError(c, errs)
	}

	result, err := h.service.Execute(c.Context(), scriptID, userID, teamID, &req)
	if err != nil {
		if errors.Is(err, repositories.ErrScriptNotFound) {
			return response.NotFound(c, "Script not found")
		}

		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "Script execution started", result)
}

// ListExecutions returns executions for a script
func (h *ScriptHandler) ListExecutions(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)
	teamID := c.Locals("teamID").(string)
	scriptID := c.Params("id")

	executions, err := h.service.ListExecutions(c.Context(), scriptID, userID, teamID)
	if err != nil {
		if errors.Is(err, repositories.ErrScriptNotFound) {
			return response.NotFound(c, "Script not found")
		}

		return response.InternalError(c, "Failed to fetch executions")
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
		return response.Error(c, fiber.StatusBadRequest, "Invalid execution ID")
	}

	execution, err := h.service.GetExecution(c.Context(), executionID)
	if err != nil {
		if errors.Is(err, repositories.ErrExecutionNotFound) {
			return response.NotFound(c, "Execution not found")
		}

		return response.InternalError(c, "Failed to fetch execution")
	}

	return response.OK(c, "Execution retrieved", dto.ToExecutionResponse(execution))
}
