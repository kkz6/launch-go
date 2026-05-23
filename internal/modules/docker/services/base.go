package services

import (
	backuprepos "github.com/kkz6/launch-go/internal/modules/backup/repositories"
	"github.com/kkz6/launch-go/internal/modules/docker/repositories"
	serverrepos "github.com/kkz6/launch-go/internal/modules/server/repositories"
	"github.com/kkz6/launch-go/internal/pkg/service"
)

// ServiceDeps holds the dependencies for docker-module services.
type ServiceDeps struct {
	service.ModuleDeps[*repositories.Registry]
	ServerRepos *serverrepos.Registry
	// BackupRepos exposes the global storage_providers repository so
	// docker database backups can reference a saved provider instead of
	// embedding S3 credentials per database. nil-tolerant — if a wiring
	// stage hasn't populated it, BackupService falls back to a clear
	// "storage providers unavailable" error.
	BackupRepos *backuprepos.Registry
}

// BaseService is the shared dependency carrier for every docker service.
type BaseService struct {
	*service.ModuleBase[*repositories.Registry]
	serviceDeps *ServiceDeps
}

// NewBaseService builds the carrier from typed deps.
func NewBaseService(deps *ServiceDeps) *BaseService {
	return &BaseService{
		ModuleBase:  service.NewModuleBase(&deps.ModuleDeps),
		serviceDeps: deps,
	}
}

// ServiceDeps returns the typed dependencies.
func (s *BaseService) ServiceDeps() *ServiceDeps { return s.serviceDeps }

// ServerRepos returns the server module's repository registry — used for
// cross-module checks (e.g. "does this server exist and belong to the
// caller's team?").
func (s *BaseService) ServerRepos() *serverrepos.Registry {
	return s.serviceDeps.ServerRepos
}

// BackupRepos returns the backup module's repository registry, used by
// BackupService to look up the saved storage_provider rows that docker
// database backups reference.
func (s *BaseService) BackupRepos() *backuprepos.Registry {
	return s.serviceDeps.BackupRepos
}
