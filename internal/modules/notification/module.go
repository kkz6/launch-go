package notification

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/middleware"
	"github.com/kkz6/launch-go/internal/modules/notification/channels"
	"github.com/kkz6/launch-go/internal/modules/notification/handlers"
	"github.com/kkz6/launch-go/internal/modules/notification/repositories"
	"github.com/kkz6/launch-go/internal/modules/notification/services"
	"github.com/kkz6/launch-go/internal/pkg/app"
	"github.com/kkz6/launch-go/internal/pkg/module"
)

const ModuleName = "notification"

// Ensure Module implements required interfaces
var (
	_ app.Module         = (*Module)(nil)
	_ app.RouteRegistrar = (*Module)(nil)
)

// Module represents the notification module
type Module struct {
	module.Base
	handler *handlers.NotificationChannelHandler
}

// NewModule creates a new notification module
func NewModule(b *module.Builder) *Module {
	deps := b.Deps()
	repo := repositories.NewNotificationChannelRepository(deps.DB)
	httpClient := channels.NewDefaultHTTPClient()
	channelFactory := channels.NewFactory(httpClient)
	service := services.NewNotificationChannelService(repo, channelFactory, deps.Logger)
	handler := handlers.NewNotificationChannelHandler(service)

	return &Module{
		Base:    module.NewBase(ModuleName, b),
		handler: handler,
	}
}

// RegisterRoutes registers the notification routes (implements RouteRegistrar interface)
func (m *Module) RegisterRoutes(router fiber.Router, authMiddleware fiber.Handler) {
	// Settings notifications routes
	notifications := router.Group("/settings/notifications", authMiddleware, middleware.TeamScope())

	// CRUD routes
	notifications.Get("/", m.handler.Index)
	notifications.Post("/", m.handler.Store)
	notifications.Get("/:id", m.handler.Show)
	notifications.Put("/:id", m.handler.Update)
	notifications.Delete("/:id", m.handler.Destroy)

	// Additional actions
	notifications.Post("/:id/test", m.handler.Test)
	notifications.Post("/:id/default", m.handler.SetDefault)
	notifications.Post("/:id/disconnect", m.handler.Disconnect)
	notifications.Post("/:id/reconnect", m.handler.Reconnect)

	// Notification channels (available channel types)
	channelsGroup := router.Group("/notification-channels", authMiddleware, middleware.TeamScope())
	channelsGroup.Get("/", m.handler.ListChannelTypes)
}

// GetService returns the notification service (for use by other modules)
func (m *Module) GetService() *services.NotificationChannelService {
	return m.handler.GetService()
}
