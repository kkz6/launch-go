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
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.OK(c, "Dashboard data retrieved", dashboard)
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
