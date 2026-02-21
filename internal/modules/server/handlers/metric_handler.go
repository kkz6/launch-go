package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/server/dto"
	pkgdto "github.com/kkz6/launch-go/internal/pkg/dto"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// GetLatestMetric returns the latest metric for a server
func (h *Handler) GetLatestMetric(c *fiber.Ctx) error {
	teamID, serverID, err := fiberctx.GetTeamAndServerID(c)
	if err != nil {
		return err
	}

	metric, err := h.service.GetLatestMetric(c.Context(), serverID, teamID)
	if err != nil {
		return fiberctx.HandleErrorOrInternal(c, err, "Failed to fetch metric")
	}

	if metric == nil {
		return fiberctx.RespondNotFound(c, "No metrics found")
	}

	return fiberctx.OK(c, "Latest metric retrieved", dto.ToMetricResponse(metric))
}

// GetMetrics returns metrics for a server
func (h *Handler) GetMetrics(c *fiber.Ctx) error {
	teamID, serverID, err := fiberctx.GetTeamAndServerID(c)
	if err != nil {
		return err
	}

	limit := fiberctx.ParseLimit(c, 100, 1000)

	metrics, err := h.service.GetMetrics(c.Context(), serverID, teamID, nil, nil, limit)
	if err != nil {
		return fiberctx.HandleErrorOrInternal(c, err, "Failed to fetch metrics")
	}

	return fiberctx.OK(c, "Metrics retrieved", pkgdto.TransformSlice(metrics, dto.ToMetricResponse))
}
