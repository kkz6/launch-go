package services

import (
	"github.com/kkz6/launch-go/internal/modules/docker/repositories"
	serverrepos "github.com/kkz6/launch-go/internal/modules/server/repositories"
	"github.com/kkz6/launch-go/internal/pkg/service"
)

// ServiceDeps holds the dependencies for docker-module services.
type ServiceDeps struct {
	service.ModuleDeps[*repositories.Registry]
	ServerRepos *serverrepos.Registry
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
