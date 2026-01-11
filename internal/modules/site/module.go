package site

import (
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/site/handlers"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	"github.com/kkz6/launch-go/internal/modules/site/repositories"
	"github.com/kkz6/launch-go/internal/modules/site/services"
	"github.com/kkz6/launch-go/internal/queue"
	"github.com/kkz6/launch-go/internal/websocket"
)

// Module represents the site module
type Module struct {
	db     *gorm.DB
	queue  *queue.Client
	ws     *websocket.Hub
	logger *zerolog.Logger

	// Repositories
	siteRepo        *repositories.SiteRepository
	deploymentRepo  *repositories.DeploymentRepository
	certificateRepo *repositories.CertificateRepository
	queueRepo       *repositories.QueueRepository
	commandRepo     *repositories.CommandRepository
	redirectRepo    *repositories.RedirectRepository
	releaseRepo     *repositories.ReleaseRepository

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
}

// NewModule creates a new site module
func NewModule(db *gorm.DB, queueClient *queue.Client, ws *websocket.Hub, logger *zerolog.Logger) *Module {
	m := &Module{
		db:     db,
		queue:  queueClient,
		ws:     ws,
		logger: logger,
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

	// Initialize handlers
	m.siteHandler = handlers.NewSiteHandler(m.siteService)
	m.deploymentHandler = handlers.NewDeploymentHandler(m.deploymentService)
	m.sslHandler = handlers.NewSSLHandler(m.sslService)
	m.queueHandler = handlers.NewQueueHandler(m.queueService)
	m.commandHandler = handlers.NewCommandHandler(m.commandService)
	m.redirectHandler = handlers.NewRedirectHandler(m.redirectService)

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

// RegisterRoutes registers HTTP routes for the site module
func (m *Module) RegisterRoutes(router fiber.Router, authMiddleware fiber.Handler) {
	// Sites are nested under servers
	servers := router.Group("/servers/:serverId", authMiddleware)

	// Site CRUD
	sites := servers.Group("/sites")
	sites.Get("/", m.siteHandler.List)
	sites.Post("/", m.siteHandler.Create)
	sites.Get("/:id", m.siteHandler.Show)
	sites.Put("/:id", m.siteHandler.Update)
	sites.Delete("/:id", m.siteHandler.Delete)

	// Site deletion summary
	sites.Get("/:id/deletion-summary", m.siteHandler.GetDeletionSummary)

	// Deployments
	sites.Post("/:id/deploy", m.deploymentHandler.Deploy)
	sites.Get("/:id/deployments", m.deploymentHandler.ListDeployments)
	sites.Get("/:id/deployments/:deploymentId", m.deploymentHandler.ShowDeployment)
	sites.Post("/:id/rollback/:deploymentId", m.deploymentHandler.Rollback)
	sites.Delete("/:id/deployments/queued", m.deploymentHandler.CancelQueuedDeployments)

	// Auto-deployment
	sites.Post("/:id/auto-deployment/enable", m.deploymentHandler.EnableAutoDeployment)
	sites.Post("/:id/auto-deployment/disable", m.deploymentHandler.DisableAutoDeployment)

	// Auto-restart queue
	sites.Post("/:id/auto-restart-queue/enable", m.queueHandler.EnableAutoRestartQueue)
	sites.Post("/:id/auto-restart-queue/disable", m.queueHandler.DisableAutoRestartQueue)

	// Deploy token
	sites.Post("/:id/deploy-token/regenerate", m.siteHandler.RegenerateDeployToken)

	// Deployment settings
	sites.Put("/:id/deployment-settings", m.siteHandler.UpdateDeploymentSettings)

	// SSL/TLS
	sites.Put("/:id/ssl", m.sslHandler.UpdateSSL)
	sites.Get("/:id/certificates", m.sslHandler.ListCertificates)

	// Queues
	sites.Get("/:id/queues", m.queueHandler.ListQueues)
	sites.Post("/:id/queues", m.queueHandler.CreateQueue)
	sites.Delete("/:id/queues/:queueId", m.queueHandler.DeleteQueue)

	// Commands
	sites.Get("/:id/commands", m.commandHandler.ListCommands)
	sites.Post("/:id/commands", m.commandHandler.CreateCommand)

	// Redirects
	sites.Get("/:id/redirects", m.redirectHandler.ListRedirects)
	sites.Post("/:id/redirects", m.redirectHandler.CreateRedirect)
	sites.Delete("/:id/redirects/:redirectId", m.redirectHandler.DeleteRedirect)
}
