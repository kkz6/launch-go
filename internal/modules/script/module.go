package script

import (
	"github.com/hibiken/asynq"

	serverrepos "github.com/kkz6/launch-go/internal/modules/server/repositories"
	"github.com/kkz6/launch-go/internal/modules/script/jobs"
	"github.com/kkz6/launch-go/internal/modules/script/repositories"
	"github.com/kkz6/launch-go/internal/modules/script/services"
	"github.com/kkz6/launch-go/internal/pkg/app"
	"github.com/kkz6/launch-go/internal/pkg/module"
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
	module.Base
	repos       *repositories.Registry
	serverRepos *serverrepos.Registry
}

// NewModule creates a new script module
func NewModule(b *module.Builder) *Module {
	deps := b.Deps()

	return &Module{
		Base:        module.NewBase(ModuleName, b),
		repos:       repositories.NewRegistry(deps.DB),
		serverRepos: serverrepos.NewRegistry(deps.DB),
	}
}

// RegisterJobs registers background job handlers
func (m *Module) RegisterJobs(mux *asynq.ServeMux) {
	deps := m.Deps()

	jobContext := jobs.NewJobContext(
		deps.DB,
		deps.Logger,
		deps.WebSocket,
		deps.Dispatcher,
		deps.Queue,
		m.repos,
		m.serverRepos,
	)
	jobs.SetJobContext(jobContext)

	jobs.RegisterHandlers(mux)
}

// createService creates the script service
func (m *Module) createService() *services.ScriptService {
	deps := m.Deps()

	return services.NewScriptService(
		m.repos,
		m.serverRepos,
		deps.Queue,
		deps.Logger,
	)
}
