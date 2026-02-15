package platform

import (
	"context"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/platform/jobs"
	"github.com/kkz6/launch-go/internal/modules/platform/repositories"
	"github.com/kkz6/launch-go/internal/modules/platform/seeders"
	"github.com/kkz6/launch-go/internal/modules/platform/services"
	"github.com/kkz6/launch-go/internal/modules/platform/tasks"
	"github.com/kkz6/launch-go/internal/pkg/app"
)

const ModuleName = "platform"

// Ensure Module implements all required interfaces
var (
	_ app.Module                = (*Module)(nil)
	_ app.RouteRegistrar        = (*Module)(nil)
	_ app.JobRegistrar          = (*Module)(nil)
	_ app.TaskCallbackRegistrar = (*Module)(nil)
	_ app.Bootable              = (*Module)(nil)
)

// Module represents the platform module
type Module struct {
	app.Base
	repos   *repositories.Registry
	service *services.PlatformUpdateService
}

// NewModule creates a new platform module
func NewModule(b *app.Builder) *Module {
	deps := b.Deps()
	repos := repositories.NewRegistry(deps.DB)
	svc := services.NewPlatformUpdateService(services.ServiceDeps{
		Dependencies: deps.ServiceDeps(),
		Repos:        repos,
	})

	return &Module{
		Base:    app.NewBase(ModuleName, b),
		repos:   repos,
		service: svc,
	}
}

// RegisterJobs registers background job handlers (implements app.JobRegistrar)
func (m *Module) RegisterJobs(mux *asynq.ServeMux) {
	jobs.Register(mux, m.Deps(), m.repos)
}

// RegisterTaskCallbacks registers task callback handlers (implements app.TaskCallbackRegistrar)
func (m *Module) RegisterTaskCallbacks() {
	tasks.RegisterTaskCallbacks()
}

// Boot runs platform seeders on startup (implements app.Bootable)
func (m *Module) Boot() error {
	logger := m.Deps().Logger
	return seeders.SeedRenameUsername(context.Background(), m.Deps().DB, logger)
}

// Repos returns the repository registry
func (m *Module) Repos() *repositories.Registry {
	return m.repos
}

// Service returns the platform update service
func (m *Module) Service() *services.PlatformUpdateService {
	return m.service
}
