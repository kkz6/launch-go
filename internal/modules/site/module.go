package site

import (
	"github.com/gofiber/fiber/v2"
	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	dnscontracts "github.com/kkz6/launch-go/internal/modules/dns/contracts"
	gitproviders "github.com/kkz6/launch-go/internal/modules/git/providers"
	gitrepos "github.com/kkz6/launch-go/internal/modules/git/repositories"
	serverrepos "github.com/kkz6/launch-go/internal/modules/server/repositories"
	servertasks "github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/modules/site/handlers"
	"github.com/kkz6/launch-go/internal/modules/site/jobs"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	"github.com/kkz6/launch-go/internal/modules/site/repositories"
	"github.com/kkz6/launch-go/internal/modules/site/services"
	"github.com/kkz6/launch-go/internal/pkg/app"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
	"github.com/kkz6/launch-go/internal/queue"
	"github.com/kkz6/launch-go/internal/websocket"
)

// Ensure Module implements required interfaces
var (
	_ app.Module            = (*Module)(nil)
	_ app.RouteRegistrar    = (*Module)(nil)
	_ app.WebhookRegistrar  = (*Module)(nil)
	_ app.JobRegistrar      = (*Module)(nil)
)

// Module represents the site module
type Module struct {
	db         *gorm.DB
	queue      *queue.Client
	ws         *websocket.Hub
	logger     *zerolog.Logger
	dispatcher *taskrunner.Dispatcher

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

	// Handlers
	siteHandler       *handlers.SiteHandler
	deploymentHandler *handlers.DeploymentHandler
	sslHandler        *handlers.SSLHandler
	queueHandler      *handlers.QueueHandler
	commandHandler    *handlers.CommandHandler
	redirectHandler   *handlers.RedirectHandler
	fileHandler       *handlers.FileHandler
	webhookHandler    *handlers.WebhookHandler

	// Additional services
	fileService *services.FileService
}

// NewModule creates a new site module
func NewModule(db *gorm.DB, queueClient *queue.Client, ws *websocket.Hub, logger *zerolog.Logger, dispatcher *taskrunner.Dispatcher) *Module {
	m := &Module{
		db:         db,
		queue:      queueClient,
		ws:         ws,
		logger:     logger,
		dispatcher: dispatcher,
	}

	// Initialize repositories
	m.siteRepo = repositories.NewSiteRepository(db)
	m.deploymentRepo = repositories.NewDeploymentRepository(db)
	m.certificateRepo = repositories.NewCertificateRepository(db)
	m.queueRepo = repositories.NewQueueRepository(db)
	m.commandRepo = repositories.NewCommandRepository(db)
	m.redirectRepo = repositories.NewRedirectRepository(db)
	m.releaseRepo = repositories.NewReleaseRepository(db)

	// Initialize services
	m.siteService = services.NewSiteService(
		m.siteRepo,
		m.deploymentRepo,
		m.certificateRepo,
		m.queueRepo,
		m.commandRepo,
		m.redirectRepo,
		m.releaseRepo,
		queueClient,
		ws,
		logger,
	)

	m.deploymentService = services.NewDeploymentService(
		m.siteRepo,
		m.deploymentRepo,
		m.certificateRepo,
		m.queueRepo,
		m.commandRepo,
		m.redirectRepo,
		m.releaseRepo,
		queueClient,
		ws,
		logger,
	)

	// Wire circular dependency
	m.siteService.SetDeploymentService(m.deploymentService)

	// Wire server repository for cross-module queries (PHP versions via relationship)
	m.serverRepo = serverrepos.NewRepository(db)
	m.siteService.SetServerRepository(m.serverRepo)

	// Initialize git source control repo
	m.sourceControlRepo = gitrepos.NewSourceControlRepository(db)

	m.sslService = services.NewSSLService(
		m.siteRepo,
		m.deploymentRepo,
		m.certificateRepo,
		m.queueRepo,
		m.commandRepo,
		m.redirectRepo,
		m.releaseRepo,
		queueClient,
		ws,
		logger,
	)

	m.queueService = services.NewQueueService(
		m.siteRepo,
		m.deploymentRepo,
		m.certificateRepo,
		m.queueRepo,
		m.commandRepo,
		m.redirectRepo,
		m.releaseRepo,
		queueClient,
		ws,
		logger,
	)

	m.commandService = services.NewCommandService(
		m.siteRepo,
		m.deploymentRepo,
		m.certificateRepo,
		m.queueRepo,
		m.commandRepo,
		m.redirectRepo,
		m.releaseRepo,
		queueClient,
		ws,
		logger,
	)

	m.redirectService = services.NewRedirectService(
		m.siteRepo,
		m.deploymentRepo,
		m.certificateRepo,
		m.queueRepo,
		m.commandRepo,
		m.redirectRepo,
		m.releaseRepo,
		queueClient,
		ws,
		logger,
	)

	// Initialize file service (needs db for server access)
	taskRunnerDeps := &servertasks.TaskRunnerDeps{
		DB:         db,
		Queue:      queueClient,
		Dispatcher: dispatcher,
		Logger:     logger,
	}
	m.fileService = services.NewFileService(db, m.siteRepo, logger, taskRunnerDeps)

	// Initialize handlers
	m.siteHandler = handlers.NewSiteHandler(m.siteService)
	m.deploymentHandler = handlers.NewDeploymentHandler(m.deploymentService)
	m.sslHandler = handlers.NewSSLHandler(m.sslService)
	m.queueHandler = handlers.NewQueueHandler(m.queueService)
	m.commandHandler = handlers.NewCommandHandler(m.commandService)
	m.redirectHandler = handlers.NewRedirectHandler(m.redirectService)
	m.fileHandler = handlers.NewFileHandler(m.fileService)
	m.webhookHandler = handlers.NewWebhookHandler(m.deploymentService)

	return m
}

// AutoMigrate runs database migrations for site models
func (m *Module) AutoMigrate() error {
	return m.db.AutoMigrate(
		&models.Site{},
		&models.Deployment{},
		&models.Certificate{},
		&models.Queue{},
		&models.Command{},
		&models.Redirect{},
		&models.Release{},
	)
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

// Name returns the module name (implements app.Module)
func (m *Module) Name() string {
	return "site"
}

// NewModuleFromContext creates a new site module from app context
func NewModuleFromContext(ctx *app.Context) *Module {
	return NewModule(ctx.DB, ctx.Queue, ctx.WebSocket, ctx.Logger, ctx.Dispatcher)
}

// RegisterWebhookRoutes registers webhook routes (implements app.WebhookRegistrar)
// These routes don't require authentication - they use deploy tokens for auth
func (m *Module) RegisterWebhookRoutes(router fiber.Router) {
	// Deployment webhook - triggered by git providers (GitHub, GitLab, Bitbucket)
	// URL: /deploy/:siteId/:token
	router.Post("/deploy/:siteId/:token", m.webhookHandler.DeployWebhook)
	router.Get("/deploy/:siteId/:token", m.webhookHandler.DeployWebhook) // Some providers use GET
}

// RegisterJobs registers background job handlers (implements app.JobRegistrar)
func (m *Module) RegisterJobs(mux *asynq.ServeMux) {
	// Set up job context with all dependencies
	jobContext := jobs.NewJobContext(
		m.db,
		m.logger,
		m.ws,
		m.dispatcher,
		m.queue,
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

	// Create kernel and register jobs
	kernel := pkgjobs.NewKernel(m.db, m.logger, m.ws)
	kernel.RegisterModule(jobs.Register)
	kernel.Boot(mux)
}

// SetProviderFactory sets the git provider factory for app-based authentication
func (m *Module) SetProviderFactory(factory *gitproviders.ProviderFactory) {
	m.providerFactory = factory
}

// SetDomainRepository sets the domain repository for domain verification
func (m *Module) SetDomainRepository(repo dnscontracts.DomainRepository) {
	m.siteHandler.SetDomainRepository(repo)
}
