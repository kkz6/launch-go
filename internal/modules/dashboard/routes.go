package dashboard

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/middleware"
	"github.com/kkz6/launch-go/internal/modules/dashboard/handlers"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// RegisterRoutes registers all dashboard routes
func (m *Module) RegisterRoutes(router fiber.Router, authMiddleware fiber.Handler) {
	handler := handlers.NewDashboardHandler(m.service)

	// Dashboard route (authenticated + team scoped + subscription required)
	router.Get("/dashboard", authMiddleware, middleware.TeamScope(), middleware.VerifySubscription(), fiberutil.Handler(handler.Index))

	// Onboarding routes (authenticated only, no team scope needed)
	router.Get("/onboarding/status", authMiddleware, handler.OnboardingStatus)
	router.Post("/onboarding/complete", authMiddleware, handler.CompleteOnboarding)
}
