package handlers

import (
	"github.com/gofiber/fiber/v2"

	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// GetMetrics returns metrics for a server. Carries an optional `limit`
// query param so it does not fit IndexNested.
func (h *Handler) GetMetrics(c *fiber.Ctx) error {
	teamID, serverID, err := fiberctx.GetTeamAndServerID(c)
	if err != nil {
		return err
	}
	limit := fiberctx.ParseLimit(c, 100, 1000)
	metrics, err := h.service.GetMetrics(c.Context(), serverID, teamID, nil, nil, limit)
	if err != nil {
		return err
	}
	return fiberctx.OK(c, "Metrics retrieved", metrics)
}
