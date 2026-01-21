package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/server/dto"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/fiberutil"
	"github.com/kkz6/launch-go/internal/pkg/response"
)

// GetLatestMetric returns the latest metric for a server
func (h *Handler) GetLatestMetric(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	serverID, err := fiberctx.GetID(c)
	if err != nil {
		return err
	}

	metric, err := h.service.GetLatestMetric(c.Context(), serverID, teamID)
	if err != nil {
		return response.HandleErrorOrInternalErr(c, err, "Failed to fetch metric")
	}

	if metric == nil {
		return response.NotFound(c, "No metrics found")
	}

	return response.OK(c, "Latest metric retrieved", dto.ToMetricResponse(metric))
}

// GetMetrics returns metrics for a server
func (h *Handler) GetMetrics(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	serverID, err := fiberctx.GetID(c)
	if err != nil {
		return err
	}

	limit := fiberutil.ParseLimit(c, 100, 1000)

	metrics, err := h.service.GetMetrics(c.Context(), serverID, teamID, nil, nil, limit)
	if err != nil {
		return response.HandleErrorOrInternalErr(c, err, "Failed to fetch metrics")
	}

	result := make([]dto.MetricResponse, len(metrics))
	for i, metric := range metrics {
		result[i] = dto.ToMetricResponse(&metric)
	}

	return response.OK(c, "Metrics retrieved", result)
}
