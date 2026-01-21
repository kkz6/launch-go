package services

import (
	"github.com/kkz6/launch-go/internal/modules/notification/channels"
	"github.com/kkz6/launch-go/internal/modules/notification/repositories"
	"github.com/kkz6/launch-go/internal/pkg/service"
)

// ServiceDeps holds all dependencies needed for notification services.
// Embedding service.ModuleDeps provides common dependencies and repository access.
type ServiceDeps struct {
	service.ModuleDeps[*repositories.Registry]
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

// BaseService provides common service dependencies for the notification module.
// It wraps service.ModuleBase and adds notification-specific functionality.
type BaseService struct {
	*service.ModuleBase[*repositories.Registry]
	serviceDeps *ServiceDeps
}

// NewBaseService creates a new base service from ServiceDeps
func NewBaseService(deps *ServiceDeps) *BaseService {
	return &BaseService{
		ModuleBase:  service.NewModuleBase(&deps.ModuleDeps),
		serviceDeps: deps,
	}
}

// ServiceDeps returns the service dependencies
func (s *BaseService) ServiceDeps() *ServiceDeps {
	return s.serviceDeps
}

// ChannelFactory returns the channel factory
func (s *BaseService) ChannelFactory() *channels.Factory {
	return s.serviceDeps.ChannelFactory
}

// Services returns the service registry for accessing other services
func (s *BaseService) Services() *ServiceRegistry {
	return s.serviceDeps.registry
}
