package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/dashboard/services"
	"github.com/kkz6/launch-go/internal/pkg/response"
)

// DashboardHandler handles HTTP requests for dashboard
type DashboardHandler struct {
	service *services.DashboardService
}

// NewDashboardHandler creates a new dashboard handler
func NewDashboardHandler(service *services.DashboardService) *DashboardHandler {
	return &DashboardHandler{service: service}
}

// Index returns dashboard data for the current team
func (h *DashboardHandler) Index(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)

	dashboard, err := h.service.GetDashboard(c.Context(), teamID)
	if err != nil {
		return response.InternalError(c, err.Error())
	}

	return response.OK(c, "Dashboard data retrieved", dashboard)
}
