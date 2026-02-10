package notification

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/middleware"
	"github.com/kkz6/launch-go/internal/modules/notification/handlers"
)

// RegisterRoutes registers the notification routes (implements RouteRegistrar interface)
func (m *Module) RegisterRoutes(router fiber.Router, authMiddleware fiber.Handler) {
	// Create services
	svc := m.createServices()

	// Create handlers
	handler := handlers.NewNotificationChannelHandler(svc.NotificationChannel())

	m.registerNotificationRoutes(router, authMiddleware, handler)
	m.registerChannelTypesRoutes(router, authMiddleware, handler)
	m.registerPreferencesRoutes(router, authMiddleware, handler)
}

// registerNotificationRoutes registers notification channel CRUD routes
func (m *Module) registerNotificationRoutes(router fiber.Router, authMiddleware fiber.Handler, handler *handlers.NotificationChannelHandler) {
	notifications := router.Group("/settings/notifications", authMiddleware, middleware.TeamScope(), middleware.VerifySubscription())
	{
		// CRUD routes
		notifications.Get("/", handler.Index)
		notifications.Post("/", handler.Store)
		notifications.Get("/:id", handler.Show)
		notifications.Put("/:id", handler.Update)
		notifications.Delete("/:id", handler.Destroy)

		// Additional actions
		notifications.Post("/:id/test", handler.Test)
		notifications.Post("/:id/default", handler.SetDefault)
		notifications.Post("/:id/disconnect", handler.Disconnect)
		notifications.Post("/:id/reconnect", handler.Reconnect)
	}
}

// registerPreferencesRoutes registers notification preferences routes
func (m *Module) registerPreferencesRoutes(router fiber.Router, authMiddleware fiber.Handler, handler *handlers.NotificationChannelHandler) {
	preferences := router.Group("/settings/notification-preferences", authMiddleware, middleware.TeamScope(), middleware.VerifySubscription())
	{
		preferences.Get("/", handler.GetPreferences)
		preferences.Put("/", handler.UpdatePreferences)
	}
}

// registerChannelTypesRoutes registers notification channel types routes
func (m *Module) registerChannelTypesRoutes(router fiber.Router, authMiddleware fiber.Handler, handler *handlers.NotificationChannelHandler) {
	channelsGroup := router.Group("/notification-channels", authMiddleware, middleware.TeamScope(), middleware.VerifySubscription())
	{
		channelsGroup.Get("/", handler.ListChannelTypes)
	}
}
