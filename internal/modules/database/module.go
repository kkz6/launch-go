package database

import (
	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/database/jobs"
	"github.com/kkz6/launch-go/internal/modules/database/repositories"
	"github.com/kkz6/launch-go/internal/modules/database/services"
	"github.com/kkz6/launch-go/internal/pkg/app"
)

const ModuleName = "database"

// Ensure Module implements required interfaces
var (
	_ app.Module         = (*Module)(nil)
	_ app.RouteRegistrar = (*Module)(nil)
	_ app.JobRegistrar   = (*Module)(nil)
)

// Module represents the database module
type Module struct {
	app.Base
	repos   *repositories.Registry
	service *services.Service
}

// NewModule creates a new database module
func NewModule(b *app.Builder) *Module {
	deps := b.Deps()
	repos := repositories.NewRegistry(deps.DB)
	service := services.NewService(services.ServiceDeps{
		Dependencies: deps.ServiceDeps(),
		Repos:        repos,
	})

	return &Module{
		Base:    app.NewBase(ModuleName, b),
		repos:   repos,
		service: service,
	}
}

// SetServerRepository sets the server repository for cross-module queries
func (m *Module) SetServerRepository(serverRepo services.ServerRepository) {
	m.service.SetServerRepository(serverRepo)
}

// RegisterJobs registers background job handlers (implements app.JobRegistrar)
func (m *Module) RegisterJobs(mux *asynq.ServeMux) {
	jobs.Register(mux, m.Deps(), m.repos)
}

// Repos returns the repository registry
func (m *Module) Repos() *repositories.Registry {
	return m.repos
}

// Service returns the database service
func (m *Module) Service() *services.Service {
	return m.service
}
