package site

import (
	"github.com/hibiken/asynq"

	dnscontracts "github.com/kkz6/launch-go/internal/modules/dns/contracts"
	gitproviders "github.com/kkz6/launch-go/internal/modules/git/providers"
	gitrepos "github.com/kkz6/launch-go/internal/modules/git/repositories"
	serverrepos "github.com/kkz6/launch-go/internal/modules/server/repositories"
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

	// Repositories
	siteRepo          *repositories.SiteRepository
	deploymentRepo    *repositories.DeploymentRepository
	certificateRepo   *repositories.CertificateRepository
	queueRepo         *repositories.QueueRepository
	commandRepo       *repositories.CommandRepository
	redirectRepo      *repositories.RedirectRepository
	releaseRepo       *repositories.ReleaseRepository
	serverRepo        *serverrepos.Repository
	sourceControlRepo *gitrepos.SourceControlRepository

	// Git provider factory
	providerFactory *gitproviders.ProviderFactory

	// Services
	siteService       *services.SiteService
	deploymentService *services.DeploymentService
	sslService        *services.SSLService
	queueService      *services.QueueService
	commandService    *services.CommandService
	redirectService   *services.RedirectService
	fileService       *services.FileService

	// Domain repository for cross-module verification
	domainRepo dnscontracts.DomainRepository
}

// NewModule creates a new site module
func NewModule(b *module.Builder) *Module {
	deps := b.Deps()

	m := &Module{
		Base: module.NewBase(ModuleName, b),
	}

	// Initialize repositories
	m.siteRepo = repositories.NewSiteRepository(deps.DB)
	m.deploymentRepo = repositories.NewDeploymentRepository(deps.DB)
	m.certificateRepo = repositories.NewCertificateRepository(deps.DB)
	m.queueRepo = repositories.NewQueueRepository(deps.DB)
	m.commandRepo = repositories.NewCommandRepository(deps.DB)
	m.redirectRepo = repositories.NewRedirectRepository(deps.DB)
	m.releaseRepo = repositories.NewReleaseRepository(deps.DB)

	// Initialize services
	m.siteService = services.NewSiteService(
		m.siteRepo,
		m.deploymentRepo,
		m.certificateRepo,
		m.queueRepo,
		m.commandRepo,
		m.redirectRepo,
		m.releaseRepo,
		deps.Queue,
		deps.WebSocket,
		deps.Logger,
	)

	m.deploymentService = services.NewDeploymentService(
		m.siteRepo,
		m.deploymentRepo,
		m.certificateRepo,
		m.queueRepo,
		m.commandRepo,
		m.redirectRepo,
		m.releaseRepo,
		deps.Queue,
		deps.WebSocket,
		deps.Logger,
	)

	// Wire circular dependency
	m.siteService.SetDeploymentService(m.deploymentService)

	// Wire server repository for cross-module queries (PHP versions via relationship)
	m.serverRepo = serverrepos.NewRepository(deps.DB)
	m.siteService.SetServerRepository(m.serverRepo)

	// Initialize git source control repo
	m.sourceControlRepo = gitrepos.NewSourceControlRepository(deps.DB)

	m.sslService = services.NewSSLService(
		m.siteRepo,
		m.deploymentRepo,
		m.certificateRepo,
		m.queueRepo,
		m.commandRepo,
		m.redirectRepo,
		m.releaseRepo,
		deps.Queue,
		deps.WebSocket,
		deps.Logger,
	)

	m.queueService = services.NewQueueService(
		m.siteRepo,
		m.deploymentRepo,
		m.certificateRepo,
		m.queueRepo,
		m.commandRepo,
		m.redirectRepo,
		m.releaseRepo,
		deps.Queue,
		deps.WebSocket,
		deps.Logger,
	)

	m.commandService = services.NewCommandService(
		m.siteRepo,
		m.deploymentRepo,
		m.certificateRepo,
		m.queueRepo,
		m.commandRepo,
		m.redirectRepo,
		m.releaseRepo,
		deps.Queue,
		deps.WebSocket,
		deps.Logger,
	)

	m.redirectService = services.NewRedirectService(
		m.siteRepo,
		m.deploymentRepo,
		m.certificateRepo,
		m.queueRepo,
		m.commandRepo,
		m.redirectRepo,
		m.releaseRepo,
		deps.Queue,
		deps.WebSocket,
		deps.Logger,
	)

	// Initialize file service (needs db for server access)
	m.fileService = services.NewFileService(deps.DB, m.siteRepo, deps.Logger, nil)

	return m
}

// SiteRepository returns the site repository instance
func (m *Module) SiteRepository() *repositories.SiteRepository {
	return m.siteRepo
}

// DeploymentRepository returns the deployment repository instance
func (m *Module) DeploymentRepository() *repositories.DeploymentRepository {
	return m.deploymentRepo
}

// SiteService returns the site service instance
func (m *Module) SiteService() *services.SiteService {
	return m.siteService
}

// DeploymentService returns the deployment service instance
func (m *Module) DeploymentService() *services.DeploymentService {
	return m.deploymentService
}

// SSLService returns the SSL service instance
func (m *Module) SSLService() *services.SSLService {
	return m.sslService
}

// QueueService returns the queue service instance
func (m *Module) QueueService() *services.QueueService {
	return m.queueService
}

// CommandService returns the command service instance
func (m *Module) CommandService() *services.CommandService {
	return m.commandService
}

// RedirectService returns the redirect service instance
func (m *Module) RedirectService() *services.RedirectService {
	return m.redirectService
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
		m.siteRepo,
		m.commandRepo,
		m.deploymentRepo,
		m.certificateRepo,
		m.queueRepo,
		m.redirectRepo,
		m.releaseRepo,
		m.serverRepo,
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

// GetDomainRepository returns the domain repository for handlers
func (m *Module) GetDomainRepository() dnscontracts.DomainRepository {
	return m.domainRepo
}
