package server

import (
	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/jobs"
	"github.com/kkz6/launch-go/internal/modules/server/repositories"
	"github.com/kkz6/launch-go/internal/modules/server/services"
	"github.com/kkz6/launch-go/internal/pkg/app"
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
	repos   *repositories.Registry
	service *services.Service
}

// NewModule creates a new server module using the builder
func NewModule(b *app.Builder) *Module {
	repos := repositories.NewRegistry(b.DB())
	service := services.NewService(services.ServiceDeps{
		Dependencies: b.ServiceDeps(),
		Repos:        repos,
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

// Repos returns the repository registry
func (m *Module) Repos() *repositories.Registry {
	return m.repos
}
