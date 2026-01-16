package services

import (
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/dns/repositories"
	apperrors "github.com/kkz6/launch-go/internal/pkg/errors"
)

// Service errors - re-exported from centralized error package
var (
	ErrProviderNotFound         = apperrors.ErrDNSProviderNotFound
	ErrDomainNotFound           = apperrors.ErrDomainNotFound
	ErrRecordNotFound           = apperrors.ErrDNSRecordNotFound
	ErrRecordNotEditable        = apperrors.BadRequest("Record cannot be edited")
	ErrRecordNotDeletable       = apperrors.BadRequest("Record cannot be deleted")
	ErrProviderHasActiveDomains = apperrors.Conflict("Provider has active domains")
	ErrInvalidCredentials       = apperrors.BadRequest("Invalid credentials")
)

// ServiceDeps holds all dependencies needed for DNS services
type ServiceDeps struct {
	DB     *gorm.DB
	Logger *zerolog.Logger
	Repos  *repositories.Registry

	// Service registry - allows services to access other services
	registry *ServiceRegistry
}

// ServiceRegistry holds all services for cross-service access
type ServiceRegistry struct {
	provider *DomainProviderService
	domain   *DomainService
	record   *DnsRecordService
}

// Provider returns the domain provider service
func (r *ServiceRegistry) Provider() *DomainProviderService { return r.provider }

// Domain returns the domain service
func (r *ServiceRegistry) Domain() *DomainService { return r.domain }

// Record returns the DNS record service
func (r *ServiceRegistry) Record() *DnsRecordService { return r.record }

// NewServiceRegistry creates all services and wires them together
func NewServiceRegistry(deps *ServiceDeps) *ServiceRegistry {
	registry := &ServiceRegistry{}
	deps.registry = registry

	// Create all services
	registry.provider = NewDomainProviderService(deps)
	registry.domain = NewDomainService(deps)
	registry.record = NewDnsRecordService(deps)

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

// Services returns the service registry for accessing other services
func (s *BaseService) Services() *ServiceRegistry {
	return s.deps.registry
}
