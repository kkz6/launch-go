package handlers

import (
	"strconv"

	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/server/dto"
	"github.com/kkz6/launch-go/internal/pkg/response"
)

// ListTasks returns all tasks for a server
func (h *Handler) ListTasks(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	serverID := c.Params("id")
	limit, _ := strconv.Atoi(c.Query("limit", "50"))

	if limit < 1 || limit > 100 {
		limit = 50
	}

	tasks, err := h.service.ListTasks(c.Context(), serverID, teamID, limit)
	if err != nil {
		return response.HandleErrorOrInternalErr(c, err, "Failed to fetch tasks")
	}

	result := make([]dto.TaskResponse, len(tasks))
	for i, task := range tasks {
		result[i] = dto.ToTaskResponse(&task)
	}

	return response.OK(c, "Tasks retrieved", result)
}

// GetLatestTask returns the latest task for a server
func (h *Handler) GetLatestTask(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	serverID := c.Params("id")

	task, err := h.service.GetLatestTask(c.Context(), serverID, teamID)
	if err != nil {
		return response.HandleErrorOrInternalErr(c, err, "Failed to fetch task")
	}

	if task == nil {
		return response.NotFound(c, "No tasks found")
	}

	return response.OK(c, "Latest task retrieved", dto.ToTaskResponse(task))
}
