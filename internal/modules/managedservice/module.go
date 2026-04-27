package managedservice

import (
	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/managedservice/contracts"
	"github.com/kkz6/launch-go/internal/modules/managedservice/jobs"
	"github.com/kkz6/launch-go/internal/modules/managedservice/repositories"
	"github.com/kkz6/launch-go/internal/modules/managedservice/services"
	servertasks "github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/pkg/app"
)

// ModuleName names this module.
const ModuleName = "managedservice"

// Ensure Module implements required interfaces.
var (
	_ app.Module         = (*Module)(nil)
	_ app.RouteRegistrar = (*Module)(nil)
	_ app.JobRegistrar   = (*Module)(nil)
)

// Module wires managed-service repositories, services, and jobs into the
// application kernel.
type Module struct {
	app.Base
	repos   *repositories.Registry
	service *services.Service
}

// NewModule constructs the managed-service module.
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

// RegisterJobs registers async job handlers.
func (m *Module) RegisterJobs(mux *asynq.ServeMux) {
	jobs.Register(mux, m.Deps(), m.repos)
}

// Repos exposes the repository registry.
func (m *Module) Repos() *repositories.Registry { return m.repos }

// Service exposes the service for cross-module wiring.
func (m *Module) Service() *services.Service { return m.service }
