package services

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/dns/providers"
	"github.com/kkz6/launch-go/internal/modules/dns/repositories"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/service"
)

// Service-level sentinel errors. Each carries its final HTTP status and
// message so the global error handler can render it without per-handler
// branching. New sentinels MUST follow the same convention.
var (
	ErrRecordNotEditable        = fiberutil.Forbidden("This record type cannot be edited")
	ErrRecordNotDeletable       = fiberutil.Forbidden("This record type cannot be deleted")
	ErrProviderHasActiveDomains = fiberutil.BadRequest("Cannot delete provider with active domains")
	ErrInvalidCredentials       = fiberutil.BadRequest("Invalid credentials")
)

// notFoundAs is a thin alias for fiberutil.NotFoundAs kept for call-site
// readability. New module code may use fiberutil.NotFoundAs directly.
func notFoundAs(err error, message string) error {
	return fiberutil.NotFoundAs(err, message)
}

// wrapProviderErr converts a *providers.ProviderError into a fiber.Error so
// it surfaces with the right HTTP status. Non-provider errors pass through
// unchanged.
func wrapProviderErr(err error) error {
	var pe *providers.ProviderError
	if !errors.As(err, &pe) {
		return err
	}
	switch pe.Code {
	case fiber.StatusNotFound:
		return fiberutil.NotFound(pe.Message)
	case fiber.StatusForbidden:
		return fiberutil.Forbidden(pe.Message)
	default:
		return fiber.NewError(fiber.StatusBadGateway, pe.Message)
	}
}

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
