// Package docker is the docker-workloads module: projects, applications,
// compose stacks, and managed databases that live on a docker-type server.
//
// Phase 1 ships projects CRUD only. Applications/Compose/Databases are
// modelled (so foreign keys land cleanly) but their services and worker
// pipelines come in later phases — see
// docs/plans/2026-05-22-docker-server-menus-design.md.
package docker

import (
	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/docker/jobs"
	"github.com/kkz6/launch-go/internal/modules/docker/repositories"
	"github.com/kkz6/launch-go/internal/modules/docker/services"
	serverrepos "github.com/kkz6/launch-go/internal/modules/server/repositories"
	"github.com/kkz6/launch-go/internal/pkg/app"
	"github.com/kkz6/launch-go/internal/pkg/service"
)

// ModuleName is the registered name for this module.
const ModuleName = "docker"

var (
	_ app.Module         = (*Module)(nil)
	_ app.RouteRegistrar = (*Module)(nil)
	_ app.JobRegistrar   = (*Module)(nil)
)

// Module wires the docker module into the framework.
type Module struct {
	app.Base
	repos       *repositories.Registry
	serverRepos *serverrepos.Registry
}

// NewModule constructs the module. ServerRepos is created here (not
// injected) so the module owns its dependency graph; if a cross-module
// access becomes shared, refactor to inject through Builder.SetX.
func NewModule(b *app.Builder) *Module {
	deps := b.Deps()
	return &Module{
		Base:        app.NewBase(ModuleName, b),
		repos:       repositories.NewRegistry(deps.DB),
		serverRepos: serverrepos.NewRegistry(deps.DB),
	}
}

// Repos exposes the repository registry for cross-module access.
func (m *Module) Repos() *repositories.Registry { return m.repos }

// RegisterJobs implements app.JobRegistrar. Binds every docker asynq task
// type to its handler.
func (m *Module) RegisterJobs(mux *asynq.ServeMux) {
	jobs.Register(mux, m.Deps(), m.repos, m.serverRepos)
}

// newProjectService builds the project service once per request boot.
func (m *Module) newProjectService() *services.ProjectService {
	return services.NewProjectService(m.serviceDeps())
}

// newApplicationService builds the application service.
func (m *Module) newApplicationService() *services.ApplicationService {
	return services.NewApplicationService(m.serviceDeps())
}

// newDomainService builds the application-domain service.
func (m *Module) newDomainService() *services.DomainService {
	return services.NewDomainService(m.serviceDeps())
}

func (m *Module) serviceDeps() *services.ServiceDeps {
	deps := m.Deps()
	return &services.ServiceDeps{
		ModuleDeps: service.ModuleDeps[*repositories.Registry]{
			Dependencies: deps.ServiceDeps(),
			Repos:        m.repos,
		},
		ServerRepos: m.serverRepos,
	}
}
