package services

import (
	"context"
	"errors"

	"github.com/rs/zerolog"

	"github.com/kkz6/launch-go/internal/modules/database/repositories"
	"github.com/kkz6/launch-go/internal/pkg/service"
	"github.com/kkz6/launch-go/internal/queue"
	"github.com/kkz6/launch-go/internal/websocket"
)

// Service-specific errors
var (
	ErrServerNotFound           = errors.New("server not found")
	ErrDatabaseNameExists       = errors.New("a database with this name already exists on this server")
	ErrDatabaseUserNameExists   = errors.New("a database user with this name already exists on this server")
	ErrInvalidExistingUser      = errors.New("the specified existing user was not found")
	ErrDatabaseBeingUninstalled = errors.New("database is being uninstalled")
	ErrUserBeingUninstalled     = errors.New("user is being uninstalled")
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
