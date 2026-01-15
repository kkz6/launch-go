package notification

import (
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/config"
	"github.com/kkz6/launch-go/internal/middleware"
	"github.com/kkz6/launch-go/internal/modules/notification/channels"
	"github.com/kkz6/launch-go/internal/modules/notification/handlers"
	"github.com/kkz6/launch-go/internal/modules/notification/models"
	"github.com/kkz6/launch-go/internal/modules/notification/repositories"
	"github.com/kkz6/launch-go/internal/modules/notification/services"
	"github.com/kkz6/launch-go/internal/pkg/app"
)

// Module represents the notification module
type Module struct {
	handler *handlers.NotificationChannelHandler
	config  *config.Config
}

// Name returns the module name
func (m *Module) Name() string {
	return "notification"
}

// NewModule creates a new notification module
func NewModule(db *gorm.DB, cfg *config.Config, logger *zerolog.Logger) *Module {
	repo := repositories.NewNotificationChannelRepository(db)
	httpClient := channels.NewDefaultHTTPClient()
	channelFactory := channels.NewFactory(httpClient)
	service := services.NewNotificationChannelService(repo, channelFactory, logger)
	handler := handlers.NewNotificationChannelHandler(service)

	return &Module{
		handler: handler,
		config:  cfg,
	}
}

// NewModuleFromContext creates a new notification module from application context
func NewModuleFromContext(ctx *app.Context) *Module {
	return NewModule(ctx.DB, ctx.Config, ctx.Logger)
}

// NewModuleWithDependencies creates a new notification module with custom dependencies (for testing)
func NewModuleWithDependencies(
	repo *repositories.NotificationChannelRepository,
	channelFactory *channels.Factory,
	cfg *config.Config,
	logger *zerolog.Logger,
) *Module {
	service := services.NewNotificationChannelService(repo, channelFactory, logger)
	handler := handlers.NewNotificationChannelHandler(service)

	return &Module{
		handler: handler,
		config:  cfg,
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

// AutoMigrate runs the database migrations for the notification module
func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(&models.NotificationChannel{})
}
