package services

import (
	"errors"

	"github.com/kkz6/launch-go/internal/modules/git/providers"
	"github.com/kkz6/launch-go/internal/modules/git/repositories"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/service"
)

var (
	ErrProviderNotSupported       = errors.New("provider not supported")
	ErrNoInstallationID           = errors.New("no installation ID found")
	ErrHasSites                   = fiberutil.Conflict("Cannot delete source control with associated sites")
	ErrInstallationAlreadyClaimed = fiberutil.Conflict("This installation is already connected to another team")
)

// ServiceDeps holds all dependencies needed for git services.
// Embedding service.ModuleDeps provides common dependencies and repository access.
type ServiceDeps struct {
	service.ModuleDeps[*repositories.Registry]
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

// BaseService provides common service dependencies for the git module.
// It wraps service.ModuleBase and adds git-specific functionality.
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

// ProviderFactory returns the provider factory
func (s *BaseService) ProviderFactory() *providers.ProviderFactory {
	return s.serviceDeps.ProviderFactory
}

// Services returns the service registry for accessing other services
func (s *BaseService) Services() *ServiceRegistry {
	return s.serviceDeps.registry
}
