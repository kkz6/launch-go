package services

import (
	"github.com/kkz6/launch-go/internal/modules/dns/repositories"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/service"
)

// Service errors - using fiber error utilities
var (
	ErrProviderNotFound         = fiberutil.NotFound("DNS provider not found")
	ErrDomainNotFound           = fiberutil.NotFound("Domain not found")
	ErrRecordNotFound           = fiberutil.NotFound("DNS record not found")
	ErrRecordNotEditable        = fiberutil.BadRequest("Record cannot be edited")
	ErrRecordNotDeletable       = fiberutil.BadRequest("Record cannot be deleted")
	ErrProviderHasActiveDomains = fiberutil.Conflict("Provider has active domains")
	ErrInvalidCredentials       = fiberutil.BadRequest("Invalid credentials")
)

// ServiceDeps holds all dependencies needed for DNS services.
// Embedding service.ModuleDeps provides common dependencies and repository access.
type ServiceDeps struct {
	service.ModuleDeps[*repositories.Registry]

	// Service registry - allows services to access other services
	registry *ServiceRegistry
}

// ServiceRegistry holds all services for cross-service access
type ServiceRegistry struct {
	provider *DomainProviderService
	domain   *DomainService
	record   *DNSRecordService
}

// Provider returns the domain provider service
func (r *ServiceRegistry) Provider() *DomainProviderService { return r.provider }

// Domain returns the domain service
func (r *ServiceRegistry) Domain() *DomainService { return r.domain }

// Record returns the DNS record service
func (r *ServiceRegistry) Record() *DNSRecordService { return r.record }

// NewServiceRegistry creates all services and wires them together
func NewServiceRegistry(deps *ServiceDeps) *ServiceRegistry {
	registry := &ServiceRegistry{}
	deps.registry = registry

	// Create all services
	registry.provider = NewDomainProviderService(deps)
	registry.domain = NewDomainService(deps)
	registry.record = NewDNSRecordService(deps)

	return registry
}

// BaseService provides common service dependencies for the DNS module.
// It wraps service.ModuleBase and adds DNS-specific functionality like Services() accessor.
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

// Services returns the service registry for accessing other services
func (s *BaseService) Services() *ServiceRegistry {
	return s.serviceDeps.registry
}
