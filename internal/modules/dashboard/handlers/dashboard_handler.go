package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/dashboard/services"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
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
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	dashboard, err := h.service.GetDashboard(c.Context(), teamID)
	if err != nil {
		return fiberctx.RespondInternalError(c, fiberctx.MsgInternalError)
	}

	return fiberctx.OK(c, "Dashboard data retrieved", dashboard)
}
