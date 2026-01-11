package site

import (
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/queue"
	"github.com/kkz6/launch-go/internal/websocket"
)

// Module represents the site module
type Module struct {
	db      *gorm.DB
	repo    *Repository
	service *Service
	handler *Handler
	queue   *queue.Client
	ws      *websocket.Hub
	logger  *zerolog.Logger
}

// NewModule creates a new site module
func NewModule(db *gorm.DB, queueClient *queue.Client, ws *websocket.Hub, logger *zerolog.Logger) *Module {
	repo := NewRepository(db)
	service := NewService(repo, queueClient, ws, logger)
	handler := NewHandler(service)

	return &Module{
		db:      db,
		repo:    repo,
		service: service,
		handler: handler,
		queue:   queueClient,
		ws:      ws,
		logger:  logger,
	}
}

// AutoMigrate runs database migrations for site models
func (m *Module) AutoMigrate() error {
	return m.db.AutoMigrate(
		&Site{},
		&Deployment{},
		&Certificate{},
		&Queue{},
		&Command{},
		&Redirect{},
		&Release{},
	)
}

// Repository returns the repository instance
func (m *Module) Repository() *Repository {
	return m.repo
}

// Service returns the service instance
func (m *Module) Service() *Service {
	return m.service
}

// Handler returns the handler instance
func (m *Module) Handler() *Handler {
	return m.handler
}

// RegisterRoutes registers HTTP routes for the site module
func (m *Module) RegisterRoutes(router fiber.Router, authMiddleware fiber.Handler) {
	// Sites are nested under servers
	servers := router.Group("/servers/:serverId", authMiddleware)

	// Site CRUD
	sites := servers.Group("/sites")
	sites.Get("/", m.handler.List)
	sites.Post("/", m.handler.Create)
	sites.Get("/:id", m.handler.Show)
	sites.Put("/:id", m.handler.Update)
	sites.Delete("/:id", m.handler.Delete)

	// Site deletion summary
	sites.Get("/:id/deletion-summary", m.handler.GetDeletionSummary)

	// Deployments
	sites.Post("/:id/deploy", m.handler.Deploy)
	sites.Get("/:id/deployments", m.handler.ListDeployments)
	sites.Get("/:id/deployments/:deploymentId", m.handler.ShowDeployment)
	sites.Post("/:id/rollback/:deploymentId", m.handler.Rollback)
	sites.Delete("/:id/deployments/queued", m.handler.CancelQueuedDeployments)

	// Auto-deployment
	sites.Post("/:id/auto-deployment/enable", m.handler.EnableAutoDeployment)
	sites.Post("/:id/auto-deployment/disable", m.handler.DisableAutoDeployment)

	// Auto-restart queue
	sites.Post("/:id/auto-restart-queue/enable", m.handler.EnableAutoRestartQueue)
	sites.Post("/:id/auto-restart-queue/disable", m.handler.DisableAutoRestartQueue)

	// Deploy token
	sites.Post("/:id/deploy-token/regenerate", m.handler.RegenerateDeployToken)

	// Deployment settings
	sites.Put("/:id/deployment-settings", m.handler.UpdateDeploymentSettings)

	// SSL/TLS
	sites.Put("/:id/ssl", m.handler.UpdateSSL)
	sites.Get("/:id/certificates", m.handler.ListCertificates)

	// Queues
	sites.Get("/:id/queues", m.handler.ListQueues)
	sites.Post("/:id/queues", m.handler.CreateQueue)
	sites.Delete("/:id/queues/:queueId", m.handler.DeleteQueue)

	// Commands
	sites.Get("/:id/commands", m.handler.ListCommands)
	sites.Post("/:id/commands", m.handler.CreateCommand)

	// Redirects
	sites.Get("/:id/redirects", m.handler.ListRedirects)
	sites.Post("/:id/redirects", m.handler.CreateRedirect)
	sites.Delete("/:id/redirects/:redirectId", m.handler.DeleteRedirect)
}
