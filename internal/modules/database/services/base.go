package services

import (
	"context"

	"github.com/rs/zerolog"

	"github.com/kkz6/launch-go/internal/modules/database/repositories"
	"github.com/kkz6/launch-go/internal/pkg/broadcast"
	apperrors "github.com/kkz6/launch-go/internal/pkg/errors"
	"github.com/kkz6/launch-go/internal/pkg/service"
	"github.com/kkz6/launch-go/internal/pkg/queue"
)

// Service-specific errors - using centralized error package
var (
	ErrServerNotFound           = apperrors.ErrServerNotFound
	ErrDatabaseNameExists       = apperrors.Conflict("A database with this name already exists on this server")
	ErrDatabaseUserNameExists   = apperrors.Conflict("A database user with this name already exists on this server")
	ErrInvalidExistingUser      = apperrors.BadRequest("The specified existing user was not found")
	ErrDatabaseBeingUninstalled = apperrors.Conflict("Database is being uninstalled")
	ErrUserBeingUninstalled     = apperrors.Conflict("User is being uninstalled")
)

// Re-export repository errors for convenience
var (
	ErrDatabaseNotFound     = repositories.ErrDatabaseNotFound
	ErrDatabaseUserNotFound = repositories.ErrDatabaseUserNotFound
)

// ServerRepository defines the interface for server operations needed by the database service
type ServerRepository interface {
	FindByID(ctx context.Context, id string) (interface{}, error)
	FindByIDAndTeam(ctx context.Context, id, teamID string) (interface{}, error)
}

// Service provides business logic for database operations
type Service struct {
	service.Base
	repos      *repositories.Registry
	serverRepo ServerRepository
}

// NewService creates a new Service instance
func NewService(repos *repositories.Registry, serverRepo ServerRepository, q *queue.Client, ws broadcast.ModelBroadcaster, logger *zerolog.Logger) *Service {
	return &Service{
		Base:       service.NewBase(q, ws, logger),
		repos:      repos,
		serverRepo: serverRepo,
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
