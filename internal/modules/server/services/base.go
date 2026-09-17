package services

import (
	"github.com/kkz6/launch-go/internal/modules/server/contracts"
	"github.com/kkz6/launch-go/internal/modules/server/providers"
	servertasks "github.com/kkz6/launch-go/internal/modules/server/tasks"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/launch/activity"
	"github.com/kkz6/launch-go/internal/pkg/service"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

var (
	ErrServerNotConnected   = fiberutil.BadRequest("Server is not connected")
	ErrInvalidProvider      = fiberutil.BadRequest("Invalid server provider")
	ErrInvalidServerType    = fiberutil.BadRequest("Invalid server type")
	ErrInvalidSoftware      = fiberutil.BadRequest("Invalid software")
	ErrServiceAlreadyExists = fiberutil.Conflict("Service already installed")
	ErrServiceBusy          = fiberutil.Conflict("Service operation already in progress")
	ErrQueueNotConfigured   = fiberutil.Internal("Queue not configured")
	ErrDaemonNotInstalled   = fiberutil.BadRequest("Daemon is not installed")
)

// ServiceDeps holds all dependencies needed for server services.
// Embedding service.Dependencies provides common dependencies.
type ServiceDeps struct {
	service.Dependencies
	Repos           contracts.RepositoryRegistry
	ProviderFactory *providers.Factory
	TaskRunnerDeps  *servertasks.TaskRunnerDeps
}

// Service provides business logic for server operations
type Service struct {
	service.Base
	activity.ActivityMixin
	repos           contracts.RepositoryRegistry
	dispatcher      *taskrunner.Dispatcher
	providerFactory *providers.Factory
	taskRunnerDeps  *servertasks.TaskRunnerDeps
}

// NewService creates a new Service instance from ServiceDeps
func NewService(deps ServiceDeps) *Service {
	return &Service{
		Base:            service.NewBaseFromDeps(deps.Dependencies),
		ActivityMixin:   activity.NewActivityMixin(deps.DB, "server"),
		repos:           deps.Repos,
		dispatcher:      deps.Dispatcher,
		providerFactory: deps.ProviderFactory,
		taskRunnerDeps:  deps.TaskRunnerDeps,
	}
}

// Repos returns the repository registry for direct access when needed
func (s *Service) Repos() contracts.RepositoryRegistry {
	return s.repos
}
