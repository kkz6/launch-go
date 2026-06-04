package handlers

import (
	"strconv"

	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/script/dto"
	"github.com/kkz6/launch-go/internal/modules/script/services"
	pkgdto "github.com/kkz6/launch-go/internal/pkg/dto"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
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
func (h *ScriptHandler) List(r *fiberutil.Request) error {
	scripts, err := h.service.List(r.Context(), r.UserID, r.TeamID)
	if err != nil {
		return fiberutil.HandleError(r.Ctx, err)
	}

	results := pkgdto.ConvertSlicePtr(scripts, dto.ToScriptResponse)

	return fiberutil.OK(r.Ctx, "Scripts retrieved", results)
}

// Show returns a single script
func (h *ScriptHandler) Show(r *fiberutil.Request) error {
	scriptID := r.Params("id")

	script, err := h.service.Get(r.Context(), scriptID, r.UserID, r.TeamID)
	if err != nil {
		if fiberutil.IsNotFound(err) {
			return fiberutil.RespondNotFound(r.Ctx, "Script not found")
		}

		return fiberutil.HandleError(r.Ctx, err)
	}

	return fiberutil.OK(r.Ctx, "Script retrieved", dto.ToScriptResponse(script))
}

// Create creates a new script.
// Skipped: uses MustGetUserID only (user-scoped); stays on fiberutil.Validate.
func (h *ScriptHandler) Create(c *fiber.Ctx, req *dto.CreateScriptRequest) error {
	userID, err := fiberutil.MustGetUserID(c)
	if err != nil {
		return err
	}

	script, err := h.service.Create(c.Context(), userID, req)
	if err != nil {
		return fiberutil.HandleError(c, err)
	}

	return fiberutil.Created(c, "Script created", dto.ToScriptResponse(script))
}

// Update updates a script
func (h *ScriptHandler) Update(r *fiberutil.Request, req *dto.UpdateScriptRequest) error {
	scriptID := r.Params("id")

	script, err := h.service.Update(r.Context(), scriptID, r.UserID, r.TeamID, req)
	if err != nil {
		if fiberutil.IsNotFound(err) {
			return fiberutil.RespondNotFound(r.Ctx, "Script not found")
		}

		return fiberutil.HandleError(r.Ctx, err)
	}

	return fiberutil.OK(r.Ctx, "Script updated", dto.ToScriptResponse(script))
}

// Delete deletes a script
func (h *ScriptHandler) Delete(r *fiberutil.Request) error {
	scriptID := r.Params("id")

	if err := h.service.Delete(r.Context(), scriptID, r.UserID, r.TeamID); err != nil {
		if fiberutil.IsNotFound(err) {
			return fiberutil.RespondNotFound(r.Ctx, "Script not found")
		}

		return fiberutil.HandleError(r.Ctx, err)
	}

	return fiberutil.NoContent(r.Ctx)
}

// Execute executes a script on servers
func (h *ScriptHandler) Execute(r *fiberutil.Request, req *dto.ExecuteScriptRequest) error {
	scriptID := r.Params("id")

	result, err := h.service.Execute(r.Context(), scriptID, r.UserID, r.TeamID, req)
	if err != nil {
		if fiberutil.IsNotFound(err) {
			return fiberutil.RespondNotFound(r.Ctx, "Script not found")
		}

		return fiberutil.HandleError(r.Ctx, err)
	}

	return fiberutil.OK(r.Ctx, "Script execution started", result)
}

// ListExecutions returns executions for a script
func (h *ScriptHandler) ListExecutions(r *fiberutil.Request) error {
	scriptID := r.Params("id")

	executions, err := h.service.ListExecutions(r.Context(), scriptID, r.UserID, r.TeamID)
	if err != nil {
		if fiberutil.IsNotFound(err) {
			return fiberutil.RespondNotFound(r.Ctx, "Script not found")
		}

		return fiberutil.HandleError(r.Ctx, err)
	}

	results := pkgdto.ConvertSlicePtr(executions, dto.ToExecutionResponse)

	return fiberutil.OK(r.Ctx, "Executions retrieved", results)
}

// GetExecution returns a single execution
func (h *ScriptHandler) GetExecution(c *fiber.Ctx) error {
	executionIDStr := c.Params("id")

	executionID, err := strconv.ParseUint(executionIDStr, 10, 64)
	if err != nil {
		return fiberutil.RespondBadRequest(c, "Invalid execution ID")
	}

	execution, err := h.service.GetExecution(c.Context(), executionID)
	if err != nil {
		if fiberutil.IsNotFound(err) {
			return fiberutil.RespondNotFound(c, "Execution not found")
		}

		return fiberutil.HandleError(c, err)
	}

	return fiberutil.OK(c, "Execution retrieved", dto.ToExecutionResponse(execution))
}
