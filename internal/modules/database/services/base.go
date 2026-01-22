package services

import (
	"context"

	"github.com/kkz6/launch-go/internal/modules/database/repositories"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/service"
)

var (
	ErrDatabaseNameExists       = fiberutil.Conflict("A database with this name already exists on this server")
	ErrDatabaseUserNameExists   = fiberutil.Conflict("A database user with this name already exists on this server")
	ErrInvalidExistingUser      = fiberutil.BadRequest("The specified existing user was not found")
	ErrDatabaseBeingUninstalled = fiberutil.Conflict("Database is being uninstalled")
	ErrUserBeingUninstalled     = fiberutil.Conflict("User is being uninstalled")
)

// ServerRepository defines the interface for server operations needed by the database service
type ServerRepository interface {
	FindByID(ctx context.Context, id string) (interface{}, error)
	FindByIDAndTeam(ctx context.Context, id, teamID string) (interface{}, error)
}

// ServiceDeps holds all dependencies needed for database services
type ServiceDeps struct {
	service.Dependencies
	Repos      *repositories.Registry
	ServerRepo ServerRepository
}

// Service provides business logic for database operations
type Service struct {
	service.Base
	repos      *repositories.Registry
	serverRepo ServerRepository
}

// NewService creates a new Service instance
func NewService(deps ServiceDeps) *Service {
	return &Service{
		Base:       service.NewBaseFromDeps(deps.Dependencies),
		repos:      deps.Repos,
		serverRepo: deps.ServerRepo,
	}
}

// Repos returns the repository registry for direct access when needed
func (s *Service) Repos() *repositories.Registry {
	return s.repos
}

// SetServerRepository sets the server repository for cross-module queries
func (s *Service) SetServerRepository(serverRepo ServerRepository) {
	s.serverRepo = serverRepo
}
