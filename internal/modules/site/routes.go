package site

import (
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/middleware"
	dnscontracts "github.com/kkz6/launch-go/internal/modules/dns/contracts"
	servertasks "github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/modules/site/handlers"
)

// RegisterRoutes registers all site module routes
func (m *Module) RegisterRoutes(router fiber.Router, authMiddleware fiber.Handler) {
	deps := m.Deps()

	// Create task runner deps for file service
	taskRunnerDeps := &servertasks.TaskRunnerDeps{
		DB:          deps.DB,
		Queue:       deps.Queue,
		Dispatcher:  deps.Dispatcher,
		Logger:      deps.Logger,
		Broadcaster: deps.WebSocket,
		Notifier:    deps.Notifier,
	}

	// Create service registry
	svc := m.createServices(taskRunnerDeps)

	// Create aggregate handler with all sub-handlers
	h := handlers.NewHandler(svc)
	h.SetDomainRepository(m.domainRepo)

	// Top-level site routes (not nested under servers)
	sitesGlobal := router.Group("/sites", authMiddleware, middleware.TeamScope(), middleware.VerifySubscription())
	m.registerGlobalSiteRoutes(sitesGlobal, h.Site)

	// Sites are nested under servers (require provisioned server)
	servers := router.Group("/servers/:serverId", authMiddleware, middleware.TeamScope(), middleware.VerifySubscription())
	sites := servers.Group("/sites", middleware.RequireProvisionedServer("serverId"))

	m.registerSiteRoutes(sites, h.Site)
	m.registerDeploymentRoutes(sites, h.Deployment)
	m.registerSSLRoutes(sites, h.SSL)
	m.registerQueueRoutes(sites, h.Queue)
	m.registerCommandRoutes(sites, h.Command)
	m.registerRedirectRoutes(sites, h.Redirect)
	m.registerFileRoutes(sites, h.File)
	m.registerFeatureRoutes(sites, h.Feature)
}

// RegisterWebhookRoutes registers webhook routes (implements app.WebhookRegistrar)
// These routes don't require authentication - they use deploy tokens for auth
func (m *Module) RegisterWebhookRoutes(router fiber.Router) {
	deps := m.Deps()

	// Create minimal task runner deps (webhooks don't need file service features)
	taskRunnerDeps := &servertasks.TaskRunnerDeps{
		DB:          deps.DB,
		Queue:       deps.Queue,
		Dispatcher:  deps.Dispatcher,
		Logger:      deps.Logger,
		Broadcaster: deps.WebSocket,
		Notifier:    deps.Notifier,
	}

	svc := m.createServices(taskRunnerDeps)
	h := handlers.NewHandler(svc)

	// Deployment webhook - triggered by git providers (GitHub, GitLab, Bitbucket)
	// URL: /deploy/:siteId/:token
	rl := middleware.RateLimit(10, time.Minute)
	router.Post("/deploy/:siteId/:token", rl, h.Webhook.DeployWebhook)
	router.Get("/deploy/:siteId/:token", rl, h.Webhook.DeployWebhook) // Some providers use GET
}

// DomainRepository returns the domain repository for cross-module access
func (m *Module) DomainRepository() dnscontracts.DomainRepository {
	return m.domainRepo
}

// registerGlobalSiteRoutes registers site routes not nested under servers
func (m *Module) registerGlobalSiteRoutes(router fiber.Router, handler *handlers.SiteHandler) {
	router.Get("/create-options", handler.GetCreateOptions)
	router.Get("/verify-domain", handler.VerifyDomain)
}

// registerSiteRoutes registers site CRUD and settings routes
func (m *Module) registerSiteRoutes(router fiber.Router, handler *handlers.SiteHandler) {
	// CRUD
	router.Get("/", handler.List)
	router.Post("/", handler.Create)
	router.Get("/:id", handler.Show)
	router.Put("/:id", handler.Update)
	router.Delete("/:id", handler.Delete)

	// Site deletion resources
	router.Get("/:id/deletion-resources", handler.GetDeletionSummary)

	// Deploy token
	router.Post("/:id/deploy-token/regenerate", handler.RegenerateDeployToken)

	// Deployment settings
	router.Patch("/:id/deployment-settings", handler.UpdateDeploymentSettings)

	// Site settings page
	router.Get("/:id/settings", handler.GetSettings)
}

// registerDeploymentRoutes registers deployment-related routes
func (m *Module) registerDeploymentRoutes(router fiber.Router, handler *handlers.DeploymentHandler) {
	// Deployments
	router.Post("/:id/deploy", handler.Deploy)
	router.Get("/:id/deployments", handler.ListDeployments)
	router.Get("/:id/deployments/:deploymentId", handler.ShowDeployment)
	router.Post("/:id/rollback/:deploymentId", handler.Rollback)
	router.Delete("/:id/deployments/queued", handler.CancelQueuedDeployments)

	// Auto-deployment
	router.Post("/:id/autodeploy", handler.ToggleAutoDeployment)
	router.Post("/:id/auto-deployment/enable", handler.EnableAutoDeployment)
	router.Post("/:id/auto-deployment/disable", handler.DisableAutoDeployment)
}

// registerSSLRoutes registers SSL/TLS routes
func (m *Module) registerSSLRoutes(router fiber.Router, handler *handlers.SSLHandler) {
	router.Put("/:id/ssl", handler.UpdateSSL)
	router.Get("/:id/certificates", handler.ListCertificates)
}

// registerQueueRoutes registers queue routes
func (m *Module) registerQueueRoutes(router fiber.Router, handler *handlers.QueueHandler) {
	router.Get("/:id/queues", handler.ListQueues)
	router.Post("/:id/queues", handler.CreateQueue)
	router.Post("/:id/queues/sync", handler.SyncQueues)
	router.Patch("/:id/queues/:queueId", handler.UpdateQueue)
	router.Post("/:id/queues/:queueId/restart", handler.RestartQueue)
	router.Delete("/:id/queues/:queueId", handler.DeleteQueue)

	// Auto-restart queue
	router.Put("/:id/auto-restart-queue", handler.UpdateAutoRestartQueue)
}

// registerCommandRoutes registers command routes
func (m *Module) registerCommandRoutes(router fiber.Router, handler *handlers.CommandHandler) {
	router.Get("/:id/commands", handler.ListCommands)
	router.Post("/:id/commands", handler.CreateCommand)
	router.Delete("/:id/commands/:commandId", handler.DeleteCommand)
}

// registerRedirectRoutes registers redirect routes
func (m *Module) registerRedirectRoutes(router fiber.Router, handler *handlers.RedirectHandler) {
	router.Get("/:id/redirects", handler.ListRedirects)
	router.Post("/:id/redirects", handler.CreateRedirect)
	router.Delete("/:id/redirects/:redirectId", handler.DeleteRedirect)
}

// registerFileRoutes registers file management routes
func (m *Module) registerFileRoutes(router fiber.Router, handler *handlers.FileHandler) {
	router.Get("/:id/files", handler.ListFiles)
	router.Get("/:id/files/:file", handler.ShowFile)     // Get file content by encoded param
	router.Put("/:id/files/:file", handler.UpdateFile)   // Update file content by encoded param
	router.Patch("/:id/files/:file", handler.UpdateFile) // Update file content by encoded param (PATCH)
	router.Get("/:id/logs", handler.ListLogs)
}

// registerFeatureRoutes registers Laravel feature management routes
func (m *Module) registerFeatureRoutes(router fiber.Router, handler *handlers.FeatureHandler) {
	router.Post("/:id/features/:feature/enable", handler.EnableFeature)
	router.Post("/:id/features/:feature/disable", handler.DisableFeature)
}
