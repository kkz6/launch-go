package server

import (
	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/contracts"
	"github.com/kkz6/launch-go/internal/modules/server/jobs"
	"github.com/kkz6/launch-go/internal/modules/server/providers"
	"github.com/kkz6/launch-go/internal/modules/server/repositories"
	"github.com/kkz6/launch-go/internal/modules/server/services"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/pkg/app"
	"github.com/kkz6/launch-go/internal/pkg/launch/sshkey"
)

const ModuleName = "server"

// Ensure Module implements all required interfaces
var (
	_ app.Module           = (*Module)(nil)
	_ app.WebhookRegistrar = (*Module)(nil)
	_ app.JobRegistrar     = (*Module)(nil)
)

// Module represents the server module
type Module struct {
	app.Base
	repos      *repositories.Registry
	service    *services.Service
	siteReader contracts.SiteReader
}

// NewModule creates a new server module using the builder
func NewModule(b *app.Builder) *Module {
	deps := b.Deps()
	repos := repositories.NewRegistry(deps.DB)
	service := services.NewService(services.ServiceDeps{
		Dependencies:    deps.ServiceDeps(),
		Repos:           repos,
		ProviderFactory: providers.NewFactory(sshkey.NewGenerator()),
		TaskRunnerDeps: &tasks.TaskRunnerDeps{
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

// RegisterJobs registers background job handlers (implements app.JobRegistrar)
func (m *Module) RegisterJobs(mux *asynq.ServeMux) {
	jobs.Register(mux, m.Deps(), m.repos)
}

// SetSiteReader sets the site reader for cross-module access
func (m *Module) SetSiteReader(reader contracts.SiteReader) {
	m.siteReader = reader
}

// Repos returns the repository registry
func (m *Module) Repos() *repositories.Registry {
	return m.repos
}

// Service returns the server service
func (m *Module) Service() *services.Service {
	return m.service
}
