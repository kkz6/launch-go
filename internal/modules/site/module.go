package site

import (
	"github.com/hibiken/asynq"

	dnscontracts "github.com/kkz6/launch-go/internal/modules/dns/contracts"
	gitproviders "github.com/kkz6/launch-go/internal/modules/git/providers"
	gitrepos "github.com/kkz6/launch-go/internal/modules/git/repositories"
	serverrepos "github.com/kkz6/launch-go/internal/modules/server/repositories"
	servertasks "github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/modules/site/adapters"
	"github.com/kkz6/launch-go/internal/modules/site/contracts"
	"github.com/kkz6/launch-go/internal/modules/site/jobs"
	"github.com/kkz6/launch-go/internal/modules/site/repositories"
	"github.com/kkz6/launch-go/internal/modules/site/services"
	sitetasks "github.com/kkz6/launch-go/internal/modules/site/tasks"
	"github.com/kkz6/launch-go/internal/pkg/app"
	"github.com/kkz6/launch-go/internal/pkg/module"
	"github.com/kkz6/launch-go/internal/pkg/service"
)

const ModuleName = "site"

// Ensure Module implements required interfaces
var (
	_ app.Module                = (*Module)(nil)
	_ app.RouteRegistrar        = (*Module)(nil)
	_ app.WebhookRegistrar      = (*Module)(nil)
	_ app.JobRegistrar          = (*Module)(nil)
	_ app.TaskCallbackRegistrar = (*Module)(nil)
)

// Module represents the site module
type Module struct {
	module.Base

	// Repository registry for site module repositories
	repos *repositories.Registry

	// Cross-module repositories (used for jobs which still need concrete types)
	serverRepos *serverrepos.Registry
	gitRepos    *gitrepos.Registry

	// Interface-based cross-module dependencies
	serverReader    contracts.ServerReader
	gitReader       contracts.GitReader
	cronCreator     contracts.CronCreator
	databaseManager contracts.DatabaseManager

	// Git provider factory (set via SetProviderFactory)
	providerFactory *gitproviders.ProviderFactory

	// Domain repository for cross-module verification (set via SetDomainRepository)
	domainRepo dnscontracts.DomainRepository
}

// NewModule creates a new site module
func NewModule(b *module.Builder) *Module {
	deps := b.Deps()

	// Create concrete repositories (still needed for jobs)
	serverRepos := serverrepos.NewRegistry(deps.DB)
	gitRepos := gitrepos.NewRegistry(deps.DB)

	// Create interface adapters for services
	serverReader := adapters.NewServerReaderAdapter(serverRepos)
	gitReader := adapters.NewGitReaderAdapter(gitRepos.SourceControl(), gitRepos.SourceControlRepo())

	return &Module{
		Base:         module.NewBase(ModuleName, b),
		repos:        repositories.NewRegistry(deps.DB),
		serverRepos:  serverRepos,
		gitRepos:     gitRepos,
		serverReader: serverReader,
		gitReader:    gitReader,
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
		m.serverRepos,
		m.gitRepos.SourceControl(),
		m.providerFactory,
	)
	jobs.SetJobContext(jobContext)

	// Register job handlers
	jobs.RegisterHandlers(mux)
}

// RegisterTaskCallbacks registers task callback handlers (implements app.TaskCallbackRegistrar)
func (m *Module) RegisterTaskCallbacks() {
	sitetasks.RegisterTaskCallbacks()
}

// SetProviderFactory sets the git provider factory for app-based authentication
func (m *Module) SetProviderFactory(factory *gitproviders.ProviderFactory) {
	m.providerFactory = factory
}

// SetDomainRepository sets the domain repository for domain verification
func (m *Module) SetDomainRepository(repo dnscontracts.DomainRepository) {
	m.domainRepo = repo
}

// SetCronCreator sets the cron creator for cross-module operations
func (m *Module) SetCronCreator(creator contracts.CronCreator) {
	m.cronCreator = creator
}

// SetDatabaseManager sets the database manager for cross-module operations
func (m *Module) SetDatabaseManager(manager contracts.DatabaseManager) {
	m.databaseManager = manager
}

// createServices creates all services needed for route handlers
func (m *Module) createServices(taskRunnerDeps *servertasks.TaskRunnerDeps) *services.ServiceRegistry {
	deps := m.Deps()

	// Create shared service dependencies using standardized ModuleDeps
	svcDeps := &services.ServiceDeps{
		ModuleDeps: service.ModuleDeps[*repositories.Registry]{
			Dependencies: deps.ServiceDeps(),
			Repos:        m.repos,
		},
		TaskRunnerDeps: taskRunnerDeps,
	}

	// Create service registry - handles all service creation and wiring
	registry := services.NewServiceRegistry(svcDeps)

	// Set cross-module dependencies using interfaces
	registry.SetCrossModuleDeps(&services.CrossModuleDeps{
		ServerReader:    m.serverReader,
		GitReader:       m.gitReader,
		CronCreator:     m.cronCreator,
		DatabaseManager: m.databaseManager,
		ProviderFactory: m.providerFactory,
	})

	return registry
}
