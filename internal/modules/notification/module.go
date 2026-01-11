package notification

import (
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/config"
	"github.com/kkz6/launch-go/internal/middleware"
	"github.com/kkz6/launch-go/internal/modules/notification/channels"
)

// Module represents the notification module
type Module struct {
	handler *Handler
	config  *config.Config
}

// NewModule creates a new notification module
func NewModule(db *gorm.DB, cfg *config.Config, logger *zerolog.Logger) *Module {
	repo := NewRepository(db)
	httpClient := channels.NewDefaultHTTPClient()
	channelFactory := channels.NewFactory(httpClient)
	service := NewService(repo, channelFactory, logger)
	handler := NewHandler(service)

	return &Module{
		handler: handler,
		config:  cfg,
	}
}

// NewModuleWithDependencies creates a new notification module with custom dependencies (for testing)
func NewModuleWithDependencies(
	repo *Repository,
	channelFactory *channels.Factory,
	cfg *config.Config,
	logger *zerolog.Logger,
) *Module {
	service := NewService(repo, channelFactory, logger)
	handler := NewHandler(service)

	return &Module{
		handler: handler,
		config:  cfg,
	}
}

// RegisterRoutes registers the notification routes
func (m *Module) RegisterRoutes(router fiber.Router) {
	notifications := router.Group("/settings/notifications", middleware.Auth(m.config.JWT.Secret))

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
}

// GetService returns the notification service (for use by other modules)
func (m *Module) GetService() *Service {
	return m.handler.service
}

// AutoMigrate runs the database migrations for the notification module
func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(&NotificationChannel{})
}
