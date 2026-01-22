package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/server/dto"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// ListCrons returns all cron jobs for a server
func (h *Handler) ListCrons(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	serverID, err := fiberctx.GetID(c)
	if err != nil {
		return err
	}

	crons, err := h.service.ListCrons(c.Context(), serverID, teamID)
	if err != nil {
		return fiberctx.HandleErrorOrInternal(c, err, "Failed to fetch cron jobs")
	}

	result := make([]dto.CronResponse, len(crons))
	for i, cron := range crons {
		result[i] = dto.ToCronResponse(&cron)
	}

	return fiberctx.OK(c, "Cron jobs retrieved", result)
}

// CreateCron creates a new cron job
func (h *Handler) CreateCron(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	serverID, err := fiberctx.GetID(c)
	if err != nil {
		return err
	}

	req, err := fiberctx.MustParseAndValidate[dto.CreateCronRequest](c)
	if err != nil {
		return err
	}

	cron, err := h.service.CreateCron(c.Context(), serverID, teamID, req)
	if err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.Created(c, "Cron job created", dto.ToCronResponse(cron))
}

// UpdateCron updates a cron job
func (h *Handler) UpdateCron(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	serverID, err := fiberctx.GetID(c)
	if err != nil {
		return err
	}

	cronID, err := fiberctx.GetCronID(c)
	if err != nil {
		return err
	}

	req, err := fiberctx.MustParseAndValidate[dto.UpdateCronRequest](c)
	if err != nil {
		return err
	}

	cron, err := h.service.UpdateCron(c.Context(), serverID, teamID, cronID, req)
	if err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.OK(c, "Cron job updated", dto.ToCronResponse(cron))
}

// DeleteCron deletes a cron job
func (h *Handler) DeleteCron(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	serverID, err := fiberctx.GetID(c)
	if err != nil {
		return err
	}

	cronID, err := fiberctx.GetCronID(c)
	if err != nil {
		return err
	}

	if err := h.service.DeleteCron(c.Context(), serverID, teamID, cronID); err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.NoContent(c)
}
