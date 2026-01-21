package services

import (
	"github.com/kkz6/launch-go/internal/modules/script/repositories"
	serverrepos "github.com/kkz6/launch-go/internal/modules/server/repositories"
	"github.com/kkz6/launch-go/internal/pkg/service"
)

// ServiceDeps holds all dependencies needed for script services.
// Embedding service.ModuleDeps provides common dependencies and repository access.
type ServiceDeps struct {
	service.ModuleDeps[*repositories.Registry]
	ServerRepos *serverrepos.Registry
}

// BaseService provides common service dependencies for the script module.
// It wraps service.ModuleBase and adds script-specific functionality.
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

// ServerRepos returns the server repository registry
func (s *BaseService) ServerRepos() *serverrepos.Registry {
	return s.serviceDeps.ServerRepos
}
