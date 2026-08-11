package services

import (
	"github.com/kkz6/launch-go/internal/modules/backup/repositories"
	"github.com/kkz6/launch-go/internal/modules/backup/storage"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/service"
)

// Service-level sentinel errors carry their final HTTP status and
// message so the global error handler renders them without per-handler
// branching. The dispatch-token error renders as 403 to match the
// previous handler-level mapping.
var (
	ErrStorageProviderHasBackups = fiberutil.Conflict("Storage provider has associated backups and cannot be deleted")
	ErrInvalidStorageDriver      = fiberutil.BadRequest("Invalid storage driver")
	ErrConnectionFailed          = fiberutil.BadRequest("Failed to connect to storage provider")
	ErrInvalidDispatchToken      = fiberutil.Forbidden(fiberutil.MsgForbidden)
	ErrInvalidCronExpression     = fiberutil.BadRequest("Invalid cron expression; use five fields: minute hour day month weekday")
)

// ServiceDeps holds all dependencies needed for backup services.
// Embedding service.ModuleDeps provides common dependencies and repository access.
type ServiceDeps struct {
	service.ModuleDeps[*repositories.Registry]

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

// BaseService provides common service dependencies for the backup module.
// It wraps service.ModuleBase and adds backup-specific functionality.
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
