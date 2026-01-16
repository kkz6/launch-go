package services

import (
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/git/providers"
	"github.com/kkz6/launch-go/internal/modules/git/repositories"
	"github.com/kkz6/launch-go/internal/queue"
)

// ServiceDeps holds all dependencies needed for git services
type ServiceDeps struct {
	DB              *gorm.DB
	Logger          *zerolog.Logger
	Queue           *queue.Client
	Repos           *repositories.Registry
	ProviderFactory *providers.ProviderFactory

	// Service registry - allows services to access other services
	registry *ServiceRegistry
}

// ServiceRegistry holds all services for cross-service access
type ServiceRegistry struct {
	sourceControl *SourceControlService
}

// SourceControl returns the source control service
func (r *ServiceRegistry) SourceControl() *SourceControlService { return r.sourceControl }

// NewServiceRegistry creates all services and wires them together
func NewServiceRegistry(deps *ServiceDeps) *ServiceRegistry {
	registry := &ServiceRegistry{}
	deps.registry = registry

	// Create all services
	registry.sourceControl = NewSourceControlService(deps)

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

// Queue returns the queue client
func (s *BaseService) Queue() *queue.Client {
	return s.deps.Queue
}

// ProviderFactory returns the provider factory
func (s *BaseService) ProviderFactory() *providers.ProviderFactory {
	return s.deps.ProviderFactory
}

// Services returns the service registry for accessing other services
func (s *BaseService) Services() *ServiceRegistry {
	return s.deps.registry
}
