package handlers

import (
	"errors"
	"strconv"

	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/server/dto"
	"github.com/kkz6/launch-go/internal/modules/server/services"
	"github.com/kkz6/launch-go/internal/pkg/response"
)

// GetLatestMetric returns the latest metric for a server
func (h *Handler) GetLatestMetric(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	serverID := c.Params("id")

	metric, err := h.service.GetLatestMetric(c.Context(), serverID, teamID)
	if err != nil {
		if errors.Is(err, services.ErrServerNotFound) {
			return response.NotFound(c, "Server not found")
		}

		return response.InternalError(c, "Failed to fetch metric")
	}

	if metric == nil {
		return response.NotFound(c, "No metrics found")
	}

	return response.OK(c, "Latest metric retrieved", dto.ToMetricResponse(metric))
}

// GetMetrics returns metrics for a server
func (h *Handler) GetMetrics(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	serverID := c.Params("id")
	limit, _ := strconv.Atoi(c.Query("limit", "100"))

	if limit < 1 || limit > 1000 {
		limit = 100
	}

	metrics, err := h.service.GetMetrics(c.Context(), serverID, teamID, nil, nil, limit)
	if err != nil {
		if errors.Is(err, services.ErrServerNotFound) {
			return response.NotFound(c, "Server not found")
		}

		return response.InternalError(c, "Failed to fetch metrics")
	}

	result := make([]dto.MetricResponse, len(metrics))
	for i, metric := range metrics {
		result[i] = dto.ToMetricResponse(&metric)
	}

	return response.OK(c, "Metrics retrieved", result)
}
