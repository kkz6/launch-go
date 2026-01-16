package site

import (
	"github.com/hibiken/asynq"

	dnscontracts "github.com/kkz6/launch-go/internal/modules/dns/contracts"
	gitproviders "github.com/kkz6/launch-go/internal/modules/git/providers"
	gitrepos "github.com/kkz6/launch-go/internal/modules/git/repositories"
	serverrepos "github.com/kkz6/launch-go/internal/modules/server/repositories"
	servertasks "github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/modules/site/jobs"
	"github.com/kkz6/launch-go/internal/modules/site/repositories"
	"github.com/kkz6/launch-go/internal/modules/site/services"
	"github.com/kkz6/launch-go/internal/pkg/app"
	"github.com/kkz6/launch-go/internal/pkg/module"
)

const ModuleName = "site"

// Ensure Module implements required interfaces
var (
	_ app.Module           = (*Module)(nil)
	_ app.RouteRegistrar   = (*Module)(nil)
	_ app.WebhookRegistrar = (*Module)(nil)
	_ app.JobRegistrar     = (*Module)(nil)
)

// Module represents the site module
type Module struct {
	module.Base

	// Repository registry for site module repositories
	repos *repositories.Registry

	// Cross-module repositories
	serverRepos       *serverrepos.Registry
	sourceControlRepo *gitrepos.SourceControlRepository

	// Git provider factory (set via SetProviderFactory)
	providerFactory *gitproviders.ProviderFactory

	// Domain repository for cross-module verification (set via SetDomainRepository)
	domainRepo dnscontracts.DomainRepository
}

// NewModule creates a new site module
func NewModule(b *module.Builder) *Module {
	deps := b.Deps()

	return &Module{
		Base:              module.NewBase(ModuleName, b),
		repos:             repositories.NewRegistry(deps.DB),
		serverRepos:       serverrepos.NewRegistry(deps.DB),
		sourceControlRepo: gitrepos.NewSourceControlRepository(deps.DB),
	}
}

// SiteRepository returns the site repository instance
func (m *Module) SiteRepository() *repositories.SiteRepository {
	return m.repos.Site()
}

// RegisterJobs registers background job handlers (implements app.JobRegistrar)
func (m *Module) RegisterJobs(mux *asynq.ServeMux) {
	deps := m.Deps()

	// Set up job context with all dependencies
	jobContext := jobs.NewJobContext(
		deps.DB,
		deps.Logger,
		deps.WebSocket,
		deps.Dispatcher,
		deps.Queue,
		m.repos.Site(),
		m.repos.Command(),
		m.repos.Deployment(),
		m.repos.Certificate(),
		m.repos.Queue(),
		m.repos.Redirect(),
		m.repos.Release(),
		m.serverRepos,
		m.sourceControlRepo,
		m.providerFactory,
	)
	jobs.SetJobContext(jobContext)

	// Register job handlers
	jobs.RegisterHandlers(mux)
}

// SetProviderFactory sets the git provider factory for app-based authentication
func (m *Module) SetProviderFactory(factory *gitproviders.ProviderFactory) {
	m.providerFactory = factory
}

// SetDomainRepository sets the domain repository for domain verification
func (m *Module) SetDomainRepository(repo dnscontracts.DomainRepository) {
	m.domainRepo = repo
}

// createServices creates all services needed for route handlers
func (m *Module) createServices(taskRunnerDeps *servertasks.TaskRunnerDeps) *services.ServiceRegistry {
	deps := m.Deps()

	// Create shared service dependencies
	svcDeps := &services.ServiceDeps{
		DB:             deps.DB,
		Logger:         deps.Logger,
		Queue:          deps.Queue,
		WebSocket:      deps.WebSocket,
		TaskRunnerDeps: taskRunnerDeps,
		Repos:          m.repos,
	}

	// Create service registry - handles all service creation and wiring
	registry := services.NewServiceRegistry(svcDeps)

	// Set cross-module dependencies
	registry.SetCrossModuleDeps(&services.CrossModuleDeps{
		ServerRepos: m.serverRepos,
	})

	return registry
}
