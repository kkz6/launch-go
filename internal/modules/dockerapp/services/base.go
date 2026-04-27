package services

import (
	"github.com/kkz6/launch-go/internal/modules/dockerapp/contracts"
	servertasks "github.com/kkz6/launch-go/internal/modules/server/tasks"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/service"
)

var (
	// ErrNameTaken is returned when an app with the given name already
	// exists on the server.
	ErrNameTaken = fiberutil.Conflict("An application with this name already exists on this server")
	// ErrBusy is returned when a lifecycle op runs against an app that
	// is mid-deploy or pending.
	ErrBusy = fiberutil.Conflict("Application is busy with another operation")
	// ErrNotDeployed is returned by lifecycle ops when no container
	// exists yet (failed deploy, never deployed).
	ErrNotDeployed = fiberutil.BadRequest("Application has not been deployed yet")
	// ErrDomainTaken is returned when adding a domain that is already
	// routed to another app.
	ErrDomainTaken = fiberutil.Conflict("This domain is already used by another application")
)

// ServiceDeps bundles the dependencies the service needs.
type ServiceDeps struct {
	service.Dependencies
	Repos          contracts.RepositoryRegistry
	ServerReader   contracts.ServerReader
	TaskRunnerDeps *servertasks.TaskRunnerDeps
}

// Service holds the business logic for docker applications.
type Service struct {
	service.Base
	repos        contracts.RepositoryRegistry
	serverReader contracts.ServerReader
	runnerDeps   *servertasks.TaskRunnerDeps
}

// NewService constructs the service.
func NewService(deps ServiceDeps) *Service {
	return &Service{
		Base:         service.NewBaseFromDeps(deps.Dependencies),
		repos:        deps.Repos,
		serverReader: deps.ServerReader,
		runnerDeps:   deps.TaskRunnerDeps,
	}
}

// SetServerReader wires the server reader after construction.
func (s *Service) SetServerReader(r contracts.ServerReader) { s.serverReader = r }

// Repos returns the repository registry.
func (s *Service) Repos() contracts.RepositoryRegistry { return s.repos }
