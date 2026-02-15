package platform

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/middleware"
	"github.com/kkz6/launch-go/internal/modules/platform/handlers"
	"github.com/kkz6/launch-go/internal/modules/platform/services"
)

// RegisterRoutes registers all platform module routes
func (m *Module) RegisterRoutes(router fiber.Router, authMiddleware fiber.Handler) {
	deps := m.Deps()

	svc := services.NewPlatformUpdateService(services.ServiceDeps{
		Dependencies: deps.ServiceDeps(),
		Repos:        m.repos,
	})

	handler := handlers.NewPlatformUpdateHandler(svc)

	updates := router.Group("/platform/updates", authMiddleware, middleware.TeamScope(), middleware.VerifySubscription())
	{
		updates.Get("/", handler.ListPendingUpdates)
		updates.Get("/:id", handler.ShowUpdate)
		updates.Post("/:id/run", handler.RunUpdate)
		updates.Post("/:id/run-all", handler.RunUpdateAll)
		updates.Post("/:id/dismiss", handler.DismissBanner)
	}
}
