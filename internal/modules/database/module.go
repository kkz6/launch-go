package database

import (
	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/database/jobs"
	"github.com/kkz6/launch-go/internal/modules/database/repositories"
	"github.com/kkz6/launch-go/internal/modules/database/services"
	"github.com/kkz6/launch-go/internal/pkg/app"
	"github.com/kkz6/launch-go/internal/pkg/module"
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
	module.Base
	repo    *repositories.Repository
	service *services.Service
}

// NewModule creates a new database module
func NewModule(b *module.Builder) *Module {
	deps := b.Deps()
	repo := repositories.NewRepository(deps.DB)
	service := services.NewService(repo, nil, deps.Queue, deps.WebSocket, deps.Logger)

	return &Module{
		Base:    module.NewBase(ModuleName, b),
		repo:    repo,
		service: service,
	}
}

// SetServerRepository sets the server repository for cross-module queries
func (m *Module) SetServerRepository(serverRepo services.ServerRepository) {
	m.service.SetServerRepository(serverRepo)
}

// RegisterJobs registers background job handlers (implements app.JobRegistrar)
func (m *Module) RegisterJobs(mux *asynq.ServeMux) {
	deps := m.Deps()
	jobContext := jobs.NewJobContext(deps.DB, m.repo, deps.Logger, deps.WebSocket, deps.Dispatcher, deps.Queue)
	jobs.SetJobContext(jobContext)
	jobs.RegisterHandlers(mux)
}

// Repository returns the database repository
func (m *Module) Repository() *repositories.Repository {
	return m.repo
}

// Service returns the database service
func (m *Module) Service() *services.Service {
	return m.service
}
