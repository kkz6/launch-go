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
func (h *DashboardHandler) Index(r *fiberctx.Request) error {
	dashboard, err := h.service.GetDashboard(r.Context(), r.TeamID)
	if err != nil {
		return fiberctx.HandleError(r.Ctx, err)
	}

	return fiberctx.OK(r.Ctx, "Dashboard data retrieved", dashboard)
}

// ActiveActions returns the team's in-flight work so users can keep track of
// deployments after navigating away from the resource that started them.
func (h *DashboardHandler) ActiveActions(r *fiberctx.Request) error {
	actions, err := h.service.ActiveActions(r.Context(), r.TeamID)
	if err != nil {
		return fiberctx.HandleError(r.Ctx, err)
	}
	return fiberctx.OK(r.Ctx, "Active actions retrieved", actions)
}

// OnboardingStatus returns the onboarding status for the current user
func (h *DashboardHandler) OnboardingStatus(c *fiber.Ctx) error {
	userID, err := fiberctx.MustGetUserID(c)
	if err != nil {
		return err
	}

	status, err := h.service.GetOnboardingStatus(c.Context(), userID)
	if err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.OK(c, "Onboarding status retrieved", status)
}

// CompleteOnboarding marks the current user as onboarded
func (h *DashboardHandler) CompleteOnboarding(c *fiber.Ctx) error {
	userID, err := fiberctx.MustGetUserID(c)
	if err != nil {
		return err
	}

	if err := h.service.CompleteOnboarding(c.Context(), userID); err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.OK(c, "Onboarding completed", nil)
}
