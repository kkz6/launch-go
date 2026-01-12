package services

import (
	"github.com/rs/zerolog"

	"github.com/kkz6/launch-go/internal/modules/server/contracts"
	"github.com/kkz6/launch-go/internal/modules/server/repositories"
	"github.com/kkz6/launch-go/internal/pkg/response"
	"github.com/kkz6/launch-go/internal/pkg/service"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
	"github.com/kkz6/launch-go/internal/queue"
	"github.com/kkz6/launch-go/internal/websocket"
)

// Service-specific errors with HTTP status codes
var (
	ErrServerNotProvisioned = response.ErrBadRequest("Server is not provisioned")
	ErrServerNotConnected   = response.ErrBadRequest("Server is not connected")
	ErrInvalidProvider      = response.ErrBadRequest("Invalid server provider")
	ErrInvalidServerType    = response.ErrBadRequest("Invalid server type")
	ErrInvalidSoftware      = response.ErrBadRequest("Invalid software")
	ErrServiceAlreadyExists = response.ErrConflict("Service already installed")
	ErrCannotDeleteService  = response.ErrBadRequest("Cannot delete service")
	ErrQueueNotConfigured   = response.ErrInternal("Queue not configured")
)

// Re-export repository errors for convenience
var (
	ErrServerNotFound       = repositories.ErrServerNotFound
	ErrServiceNotFound      = repositories.ErrServiceNotFound
	ErrFirewallRuleNotFound = repositories.ErrFirewallRuleNotFound
	ErrCronNotFound         = repositories.ErrCronNotFound
	ErrDaemonNotFound       = repositories.ErrDaemonNotFound
	ErrSshKeyNotFound       = repositories.ErrSshKeyNotFound
	ErrTaskNotFound         = repositories.ErrTaskNotFound
)

// Service provides business logic for server operations
type Service struct {
	service.Base
	repo       contracts.Repository
	dispatcher *taskrunner.Dispatcher
}

// NewService creates a new Service instance
func NewService(repo contracts.Repository, q *queue.Client, ws *websocket.Hub, dispatcher *taskrunner.Dispatcher, logger *zerolog.Logger) *Service {
	return &Service{
		Base:       service.NewBase(q, ws, logger),
		repo:       repo,
		dispatcher: dispatcher,
	}
}

// Repo returns the repository for direct access when needed
func (s *Service) Repo() contracts.Repository {
	return s.repo
}
