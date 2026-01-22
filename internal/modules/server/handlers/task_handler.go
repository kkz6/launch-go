package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/server/dto"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// ListTasks returns all tasks for a server
func (h *Handler) ListTasks(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	serverID, err := fiberctx.GetID(c)
	if err != nil {
		return err
	}

	limit := fiberctx.ParseLimit(c, 50, 100)

	tasks, err := h.service.ListTasks(c.Context(), serverID, teamID, limit)
	if err != nil {
		return fiberctx.HandleErrorOrInternal(c, err, "Failed to fetch tasks")
	}

	result := make([]dto.TaskResponse, len(tasks))
	for i, task := range tasks {
		result[i] = dto.ToTaskResponse(&task)
	}

	return fiberctx.OK(c, "Tasks retrieved", result)
}

// GetLatestTask returns the latest task for a server
func (h *Handler) GetLatestTask(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	serverID, err := fiberctx.GetID(c)
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
