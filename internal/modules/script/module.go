package script

import (
	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/script/jobs"
	"github.com/kkz6/launch-go/internal/modules/script/repositories"
	"github.com/kkz6/launch-go/internal/modules/script/services"
	serverrepos "github.com/kkz6/launch-go/internal/modules/server/repositories"
	"github.com/kkz6/launch-go/internal/pkg/app"
	"github.com/kkz6/launch-go/internal/pkg/service"
)

const ModuleName = "script"

// Ensure Module implements required interfaces
var (
	_ app.Module         = (*Module)(nil)
	_ app.RouteRegistrar = (*Module)(nil)
	_ app.JobRegistrar   = (*Module)(nil)
)

// Module represents the script module
type Module struct {
	app.Base
	repos       *repositories.Registry
	serverRepos *serverrepos.Registry
}

// NewModule creates a new script module
func NewModule(b *app.Builder) *Module {
	deps := b.Deps()

	return &Module{
		Base:        app.NewBase(ModuleName, b),
		repos:       repositories.NewRegistry(deps.DB),
		serverRepos: serverrepos.NewRegistry(deps.DB),
	}
}

// RegisterJobs registers background job handlers
func (m *Module) RegisterJobs(mux *asynq.ServeMux) {
	deps := m.Deps()

	// Register job handlers
	jobs.Register(mux, deps, m.repos, m.serverRepos)
}

// createService creates the script service
func (m *Module) createService() *services.ScriptService {
	deps := m.Deps()

	serviceDeps := &services.ServiceDeps{
		ModuleDeps: service.ModuleDeps[*repositories.Registry]{
			Dependencies: deps.ServiceDeps(),
			Repos:        m.repos,
		},
		ServerRepos: m.serverRepos,
	}

	return services.NewScriptService(serviceDeps)
}
