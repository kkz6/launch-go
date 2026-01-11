package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/server/dto"
	"github.com/kkz6/launch-go/internal/pkg/response"
	"github.com/kkz6/launch-go/internal/pkg/validator"
)

// ListCrons returns all cron jobs for a server
func (h *Handler) ListCrons(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	serverID := c.Params("id")

	crons, err := h.service.ListCrons(c.Context(), serverID, teamID)
	if err != nil {
		return response.HandleErrorOrInternalErr(c, err, "Failed to fetch cron jobs")
	}

	result := make([]dto.CronResponse, len(crons))
	for i, cron := range crons {
		result[i] = dto.ToCronResponse(&cron)
	}

	return response.OK(c, "Cron jobs retrieved", result)
}

// CreateCron creates a new cron job
func (h *Handler) CreateCron(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	serverID := c.Params("id")

	var req dto.CreateCronRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if errs := validator.Validate(&req); errs != nil {
		return response.ValidationError(c, errs)
	}

	cron, err := h.service.CreateCron(c.Context(), serverID, teamID, &req)
	if err != nil {
		return response.HandleError(c, err)
	}

	return response.Created(c, "Cron job created", dto.ToCronResponse(cron))
}

// UpdateCron updates a cron job
func (h *Handler) UpdateCron(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	serverID := c.Params("id")
	cronID := c.Params("cronId")

	var req dto.UpdateCronRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if errs := validator.Validate(&req); errs != nil {
		return response.ValidationError(c, errs)
	}

	cron, err := h.service.UpdateCron(c.Context(), serverID, teamID, cronID, &req)
	if err != nil {
		return response.HandleError(c, err)
	}

	return response.OK(c, "Cron job updated", dto.ToCronResponse(cron))
}

// DeleteCron deletes a cron job
func (h *Handler) DeleteCron(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	serverID := c.Params("id")
	cronID := c.Params("cronId")

	if err := h.service.DeleteCron(c.Context(), serverID, teamID, cronID); err != nil {
		return response.HandleError(c, err)
	}

	return response.NoContent(c)
}
