package dockerapp

import (
	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/dockerapp/contracts"
	"github.com/kkz6/launch-go/internal/modules/dockerapp/jobs"
	"github.com/kkz6/launch-go/internal/modules/dockerapp/repositories"
	"github.com/kkz6/launch-go/internal/modules/dockerapp/services"
	servertasks "github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/pkg/app"
)

// ModuleName names this module.
const ModuleName = "dockerapp"

// Ensure Module implements required interfaces.
var (
	_ app.Module         = (*Module)(nil)
	_ app.RouteRegistrar = (*Module)(nil)
	_ app.JobRegistrar   = (*Module)(nil)
)

// Module wires dockerapp repositories, services, and jobs into the
// application kernel.
type Module struct {
	app.Base
	repos    *repositories.Registry
	service  *services.Service
	registry contracts.RegistryCredentialReader
}

// NewModule constructs the dockerapp module.
func NewModule(b *app.Builder) *Module {
	deps := b.Deps()
	repos := repositories.NewRegistry(deps.DB)

	service := services.NewService(services.ServiceDeps{
		Dependencies: deps.ServiceDeps(),
		Repos:        repos,
		TaskRunnerDeps: &servertasks.TaskRunnerDeps{
			DB:          deps.DB,
			Queue:       deps.Queue,
			Dispatcher:  deps.Dispatcher,
			Logger:      deps.Logger,
			Broadcaster: deps.WebSocket,
			Notifier:    deps.Notifier,
		},
	})

	return &Module{
		Base:    app.NewBase(ModuleName, b),
		repos:   repos,
		service: service,
	}
}

// SetServerReader wires the cross-module server reader.
func (m *Module) SetServerReader(r contracts.ServerReader) {
	m.service.SetServerReader(r)
}

// SetRegistryCredentialReader wires the cross-module registry reader.
// Used by the deploy job to look up docker login credentials.
func (m *Module) SetRegistryCredentialReader(r contracts.RegistryCredentialReader) {
	m.registry = r
}

// RegisterJobs registers async job handlers.
func (m *Module) RegisterJobs(mux *asynq.ServeMux) {
	jobs.Register(mux, m.Deps(), m.repos, m.registry)
}

// Repos exposes the repository registry.
func (m *Module) Repos() *repositories.Registry { return m.repos }

// Service exposes the service for cross-module wiring.
func (m *Module) Service() *services.Service { return m.service }
