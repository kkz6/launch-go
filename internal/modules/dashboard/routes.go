package dashboard

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/middleware"
	"github.com/kkz6/launch-go/internal/modules/dashboard/handlers"
)

// RegisterRoutes registers all dashboard routes
func (m *Module) RegisterRoutes(router fiber.Router, authMiddleware fiber.Handler) {
	handler := handlers.NewDashboardHandler(m.service)

	// Dashboard route (authenticated + team scoped)
	router.Get("/dashboard", authMiddleware, middleware.TeamScope(), handler.Index)
}
