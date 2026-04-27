package services

import (
	"github.com/kkz6/launch-go/internal/modules/managedservice/contracts"
	servertasks "github.com/kkz6/launch-go/internal/modules/server/tasks"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/service"
)

var (
	// ErrAlreadyInstalled is returned when an Install request hits a
	// server that already has a service of that kind.
	ErrAlreadyInstalled = fiberutil.Conflict("This server already has a managed service of this kind")
	// ErrBusy is returned when a lifecycle op runs against a service
	// that is still installing/uninstalling.
	ErrBusy = fiberutil.Conflict("Managed service is busy with another operation")
	// ErrNotRunning is returned by lifecycle ops when the service has
	// no container to act on (failed install, etc.).
	ErrNotRunning = fiberutil.BadRequest("Managed service is not in a runnable state")
)

// ServiceDeps bundles all dependencies the managed-service service needs.
type ServiceDeps struct {
	service.Dependencies
	Repos          contracts.RepositoryRegistry
	ServerReader   contracts.ServerReader
	TaskRunnerDeps *servertasks.TaskRunnerDeps
}

// Service holds the business logic for managed services.
type Service struct {
	service.Base
	repos        contracts.RepositoryRegistry
	serverReader contracts.ServerReader
	runnerDeps   *servertasks.TaskRunnerDeps
}

// NewService constructs a managed-service service from its dependencies.
func NewService(deps ServiceDeps) *Service {
	return &Service{
		Base:         service.NewBaseFromDeps(deps.Dependencies),
		repos:        deps.Repos,
		serverReader: deps.ServerReader,
		runnerDeps:   deps.TaskRunnerDeps,
	}
}

// SetServerReader wires the cross-module server reader after construction.
func (s *Service) SetServerReader(r contracts.ServerReader) { s.serverReader = r }

// Repos returns the repository registry.
func (s *Service) Repos() contracts.RepositoryRegistry { return s.repos }
