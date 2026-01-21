package services

import (
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/backup/repositories"
	"github.com/kkz6/launch-go/internal/modules/backup/storage"
	"github.com/kkz6/launch-go/internal/pkg/service"
)

// ServiceDeps holds all dependencies needed for backup services.
// Embedding service.Dependencies provides common dependencies.
type ServiceDeps struct {
	service.Dependencies
	Repos *repositories.Registry

	// Service registry - allows services to access other services
	registry *ServiceRegistry
}

// ServiceRegistry holds all services for cross-service access
type ServiceRegistry struct {
	backup          *BackupService
	backupJob       *BackupJobService
	storageProvider *StorageProviderService
	agentConfig     *AgentConfigService
}

// Backup returns the backup service
func (r *ServiceRegistry) Backup() *BackupService { return r.backup }

// BackupJob returns the backup job service
func (r *ServiceRegistry) BackupJob() *BackupJobService { return r.backupJob }

// StorageProvider returns the storage provider service
func (r *ServiceRegistry) StorageProvider() *StorageProviderService { return r.storageProvider }

// AgentConfig returns the agent config service
func (r *ServiceRegistry) AgentConfig() *AgentConfigService { return r.agentConfig }

// NewServiceRegistry creates all services and wires them together
func NewServiceRegistry(deps *ServiceDeps) *ServiceRegistry {
	registry := &ServiceRegistry{}
	deps.registry = registry

	// Create storage factory
	storageFactory := storage.NewFactory()

	// Create all services
	registry.backup = NewBackupService(deps)
	registry.backupJob = NewBackupJobService(deps)
	registry.storageProvider = NewStorageProviderService(deps, storageFactory)
	registry.agentConfig = NewAgentConfigService(deps)

	return registry
}

// BaseService provides common service dependencies
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

// DB returns the database connection
func (s *BaseService) DB() *gorm.DB {
	return s.deps.DB
}

// ServiceDeps returns the service dependencies
func (s *BaseService) ServiceDeps() *ServiceDeps {
	return s.deps
}

// Services returns the service registry for accessing other services
func (s *BaseService) Services() *ServiceRegistry {
	return s.deps.registry
}
