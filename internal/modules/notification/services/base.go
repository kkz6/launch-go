package services

import (
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/notification/channels"
	"github.com/kkz6/launch-go/internal/modules/notification/repositories"
)

// ServiceDeps holds all dependencies needed for notification services
type ServiceDeps struct {
	DB             *gorm.DB
	Logger         *zerolog.Logger
	Repos          *repositories.Registry
	ChannelFactory *channels.Factory

	// Admin webhook URL for Slack admin alerts
	AdminWebhookURL string

	// Service registry - allows services to access other services
	registry *ServiceRegistry
}

// ServiceRegistry holds all services for cross-service access
type ServiceRegistry struct {
	notificationChannel *NotificationChannelService
	notifier            *Notifier
}

// NotificationChannel returns the notification channel service
func (r *ServiceRegistry) NotificationChannel() *NotificationChannelService {
	return r.notificationChannel
}

// Notifier returns the notifier service
func (r *ServiceRegistry) Notifier() *Notifier {
	return r.notifier
}

// NewServiceRegistry creates all services and wires them together
func NewServiceRegistry(deps *ServiceDeps) *ServiceRegistry {
	registry := &ServiceRegistry{}
	deps.registry = registry

	// Create all services
	registry.notificationChannel = NewNotificationChannelService(deps)
	registry.notifier = NewNotifier(registry.notificationChannel, deps.AdminWebhookURL)

	return registry
}

// BaseService provides common service dependencies
type BaseService struct {
	deps   *ServiceDeps
	repos  *repositories.Registry
	logger *zerolog.Logger
}

// NewBaseService creates a new base service from ServiceDeps
func NewBaseService(deps *ServiceDeps) *BaseService {
	return &BaseService{
		deps:   deps,
		repos:  deps.Repos,
		logger: deps.Logger,
	}
}

// Repos returns the repository registry
func (s *BaseService) Repos() *repositories.Registry {
	return s.repos
}

// DB returns the database connection
func (s *BaseService) DB() *gorm.DB {
	return s.deps.DB
}

// Logger returns the logger
func (s *BaseService) Logger() *zerolog.Logger {
	return s.logger
}

// ChannelFactory returns the channel factory
func (s *BaseService) ChannelFactory() *channels.Factory {
	return s.deps.ChannelFactory
}

// Services returns the service registry for accessing other services
func (s *BaseService) Services() *ServiceRegistry {
	return s.deps.registry
}
