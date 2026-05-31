package backup

import (
	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/backup/jobs"
	"github.com/kkz6/launch-go/internal/modules/backup/repositories"
	"github.com/kkz6/launch-go/internal/modules/backup/services"
	databaserepos "github.com/kkz6/launch-go/internal/modules/database/repositories"
	serverrepos "github.com/kkz6/launch-go/internal/modules/server/repositories"
	"github.com/kkz6/launch-go/internal/pkg/app"
	"github.com/kkz6/launch-go/internal/pkg/service"
)

const ModuleName = "backup"

// Ensure Module implements required interfaces
var (
	_ app.Module           = (*Module)(nil)
	_ app.RouteRegistrar   = (*Module)(nil)
	_ app.WebhookRegistrar = (*Module)(nil)
	_ app.JobRegistrar     = (*Module)(nil)
)

// Module represents the backup module
type Module struct {
	app.Base

	// Repository registry
	repos *repositories.Registry

	// Cross-module repositories (needed for backup jobs that access server data)
	serverRepos *serverrepos.Registry

	// Database module's repo registry. Optional — set via
	// SetDatabaseRepos() from the host process so manual backup runs
	// can dump the linked databases (mysqldump / pg_dump) before
	// tarring. Without it, runs are files-only.
	databaseRepos *databaserepos.Registry
}

// SetDatabaseRepos wires the database module's repository registry so
// backup jobs can resolve linked databases + database_users for dump.
// Call this from the host process after both modules are constructed.
func (m *Module) SetDatabaseRepos(repos *databaserepos.Registry) {
	m.databaseRepos = repos
}

// NewModule creates a new backup module
func NewModule(b *app.Builder) *Module {
	deps := b.Deps()

	return &Module{
		Base:        app.NewBase(ModuleName, b),
		repos:       repositories.NewRegistry(deps.DB),
		serverRepos: serverrepos.NewRegistry(deps.DB),
	}
}

// createServices creates all services needed for route handlers
func (m *Module) createServices() *services.ServiceRegistry {
	deps := m.Deps()

	// Create shared service dependencies using standardized ModuleDeps
	svcDeps := &services.ServiceDeps{
		ModuleDeps: service.ModuleDeps[*repositories.Registry]{
			Dependencies: deps.ServiceDeps(),
			Repos:        m.repos,
		},
	}

	// Create service registry - handles all service creation and wiring
	return services.NewServiceRegistry(svcDeps)
}

// RegisterJobs registers background job handlers (implements app.JobRegistrar)
func (m *Module) RegisterJobs(mux *asynq.ServeMux) {
	jobs.Register(mux, m.Deps(), m.repos, m.serverRepos, m.databaseRepos)
}

// Repos returns the repository registry
func (m *Module) Repos() *repositories.Registry {
	return m.repos
}
