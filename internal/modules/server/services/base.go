package services

import (
	"errors"

	"github.com/rs/zerolog"

	"github.com/kkz6/launch-go/internal/modules/server/contracts"
	"github.com/kkz6/launch-go/internal/modules/server/repositories"
	"github.com/kkz6/launch-go/internal/queue"
	"github.com/kkz6/launch-go/internal/taskrunner"
	"github.com/kkz6/launch-go/internal/websocket"
)

// Service-specific errors
var (
	ErrServerNotProvisioned = errors.New("server is not provisioned")
	ErrServerNotConnected   = errors.New("server is not connected")
	ErrInvalidProvider      = errors.New("invalid server provider")
	ErrInvalidServerType    = errors.New("invalid server type")
	ErrInvalidSoftware      = errors.New("invalid software")
	ErrServiceAlreadyExists = errors.New("service already exists")
	ErrCannotDeleteService  = errors.New("cannot delete service")
	ErrQueueNotConfigured   = errors.New("queue not configured")
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
	repo       contracts.Repository
	queue      *queue.Client
	ws         *websocket.Hub
	dispatcher *taskrunner.Dispatcher
	logger     *zerolog.Logger
}

// NewService creates a new Service instance
func NewService(repo contracts.Repository, queue *queue.Client, ws *websocket.Hub, dispatcher *taskrunner.Dispatcher, logger *zerolog.Logger) *Service {
	return &Service{
		repo:       repo,
		queue:      queue,
		ws:         ws,
		dispatcher: dispatcher,
		logger:     logger,
	}
}
