package services

import (
	"github.com/kkz6/launch-go/internal/modules/script/repositories"
	serverrepos "github.com/kkz6/launch-go/internal/modules/server/repositories"
	"github.com/kkz6/launch-go/internal/pkg/service"
)

// ServiceDeps holds all dependencies needed for script services.
// Embedding service.Dependencies provides common dependencies.
type ServiceDeps struct {
	service.Dependencies
	Repos       *repositories.Registry
	ServerRepos *serverrepos.Registry
}

// BaseService provides common service dependencies
type BaseService struct {
	service.Base
	deps        *ServiceDeps
	repos       *repositories.Registry
	serverRepos *serverrepos.Registry
}

// NewBaseService creates a new base service from ServiceDeps
func NewBaseService(deps *ServiceDeps) *BaseService {
	return &BaseService{
		Base:        service.NewBaseFromDeps(deps.Dependencies),
		deps:        deps,
		repos:       deps.Repos,
		serverRepos: deps.ServerRepos,
	}
}

// Repos returns the script repository registry
func (s *BaseService) Repos() *repositories.Registry {
	return s.repos
}

// ServerRepos returns the server repository registry
func (s *BaseService) ServerRepos() *serverrepos.Registry {
	return s.serverRepos
}

// ServiceDeps returns the service dependencies
func (s *BaseService) ServiceDeps() *ServiceDeps {
	return s.deps
}
