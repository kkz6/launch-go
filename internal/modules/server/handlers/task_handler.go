package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/server/dto"
	pkgdto "github.com/kkz6/launch-go/internal/pkg/dto"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// ListTasks returns all tasks for a server
func (h *Handler) ListTasks(c *fiber.Ctx) error {
	teamID, serverID, err := fiberctx.GetTeamAndServerID(c)
	if err != nil {
		return err
	}

	limit := fiberctx.ParseLimit(c, 50, 100)

	tasks, err := h.service.ListTasks(c.Context(), serverID, teamID, limit)
	if err != nil {
		return fiberctx.HandleErrorOrInternal(c, err, "Failed to fetch tasks")
	}

	return fiberctx.OK(c, "Tasks retrieved", pkgdto.TransformSlice(tasks, dto.ToTaskResponse))
}

// GetTask returns a specific task by ID
func (h *Handler) GetTask(c *fiber.Ctx) error {
	teamID, serverID, err := fiberctx.GetTeamAndServerID(c)
	if err != nil {
		return err
	}

	taskID := c.Params("taskId")
	if taskID == "" {
		return fiberctx.RespondBadRequest(c, "Missing task ID")
	}

	task, err := h.service.GetTask(c.Context(), taskID, serverID, teamID)
	if err != nil {
		return fiberctx.HandleErrorOrInternal(c, err, "Failed to fetch task")
	}

	if task == nil {
		return fiberctx.RespondNotFound(c, "Task not found")
	}

	return fiberctx.OK(c, "Task retrieved", dto.ToTaskResponse(task))
}

// GetLatestTask returns the latest task for a server
func (h *Handler) GetLatestTask(c *fiber.Ctx) error {
	teamID, serverID, err := fiberctx.GetTeamAndServerID(c)
	if err != nil {
		return err
	}

	task, err := h.service.GetLatestTask(c.Context(), serverID, teamID)
	if err != nil {
		return fiberctx.HandleErrorOrInternal(c, err, "Failed to fetch task")
	}

	if task == nil {
		return fiberctx.RespondNotFound(c, "No tasks found")
	}

	return fiberctx.OK(c, "Latest task retrieved", dto.ToTaskResponse(task))
}
