package services

import (
	"github.com/rs/zerolog"

	"github.com/kkz6/launch-go/internal/modules/notification/channels"
	"github.com/kkz6/launch-go/internal/modules/notification/repositories"
	"github.com/kkz6/launch-go/internal/pkg/service"
)

// ServiceDeps holds all dependencies needed for notification services.
// Embedding service.Dependencies provides common dependencies (DB, Logger, Queue, Broadcaster).
type ServiceDeps struct {
	service.Dependencies
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

// BaseService provides common service dependencies for notification services.
// It embeds service.Base to get common functionality (queue, broadcast, logging)
// and adds module-specific accessors.
type BaseService struct {
	service.Base
	deps  *ServiceDeps
	repos *repositories.Registry
}

// NewBaseService creates a new base service from ServiceDeps
func NewBaseService(deps *ServiceDeps) *BaseService {
	return &BaseService{
		Base:  service.NewBaseFromDeps(deps.Dependencies),
		deps:  deps,
		repos: deps.Repos,
	}
}

// Repos returns the repository registry
func (s *BaseService) Repos() *repositories.Registry {
	return s.repos
}

// Logger returns the logger (overrides Base.Logger for pointer return)
func (s *BaseService) Logger() *zerolog.Logger {
	return s.deps.Logger
}

// ChannelFactory returns the channel factory
func (s *BaseService) ChannelFactory() *channels.Factory {
	return s.deps.ChannelFactory
}

// Services returns the service registry for accessing other services
func (s *BaseService) Services() *ServiceRegistry {
	return s.deps.registry
}
