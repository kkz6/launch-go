package services

import (
	"context"

	"github.com/rs/zerolog"

	"github.com/kkz6/launch-go/internal/modules/database/repositories"
	"github.com/kkz6/launch-go/internal/pkg/response"
	"github.com/kkz6/launch-go/internal/pkg/service"
	"github.com/kkz6/launch-go/internal/queue"
	"github.com/kkz6/launch-go/internal/websocket"
)

// Service-specific errors with HTTP status codes
var (
	ErrServerNotFound           = response.ErrNotFound("Server not found")
	ErrDatabaseNameExists       = response.ErrConflict("A database with this name already exists on this server")
	ErrDatabaseUserNameExists   = response.ErrConflict("A database user with this name already exists on this server")
	ErrInvalidExistingUser      = response.ErrBadRequest("The specified existing user was not found")
	ErrDatabaseBeingUninstalled = response.ErrConflict("Database is being uninstalled")
	ErrUserBeingUninstalled     = response.ErrConflict("User is being uninstalled")
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
	repo       *repositories.Repository
	serverRepo ServerRepository
}

// NewService creates a new Service instance
func NewService(repo *repositories.Repository, serverRepo ServerRepository, q *queue.Client, ws *websocket.Hub, logger *zerolog.Logger) *Service {
	return &Service{
		Base:       service.NewBase(q, ws, logger),
		repo:       repo,
		serverRepo: serverRepo,
	}
}

// Repo returns the repository for direct access when needed
func (s *Service) Repo() *repositories.Repository {
	return s.repo
}
