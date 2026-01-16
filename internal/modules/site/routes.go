package site

import (
	"github.com/gofiber/fiber/v2"

	dnscontracts "github.com/kkz6/launch-go/internal/modules/dns/contracts"
	servertasks "github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/modules/site/handlers"
)

// RegisterRoutes registers all site module routes
func (m *Module) RegisterRoutes(router fiber.Router, authMiddleware fiber.Handler) {
	deps := m.Deps()

	// Create task runner deps for file service
	taskRunnerDeps := &servertasks.TaskRunnerDeps{
		DB:         deps.DB,
		Queue:      deps.Queue,
		Dispatcher: deps.Dispatcher,
		Logger:     deps.Logger,
	}

	// Create service registry
	svc := m.createServices(taskRunnerDeps)

	// Create handlers
	siteHandler := handlers.NewSiteHandler(svc.Site())
	siteHandler.SetDomainRepository(m.domainRepo)
	deploymentHandler := handlers.NewDeploymentHandler(svc.Deployment())
	sslHandler := handlers.NewSSLHandler(svc.SSL())
	queueHandler := handlers.NewQueueHandler(svc.Queue())
	commandHandler := handlers.NewCommandHandler(svc.Command())
	redirectHandler := handlers.NewRedirectHandler(svc.Redirect())
	fileHandler := handlers.NewFileHandler(svc.File())

	// Top-level site routes (not nested under servers)
	sitesGlobal := router.Group("/sites", authMiddleware)
	registerGlobalSiteRoutes(sitesGlobal, siteHandler)

	// Sites are nested under servers
	servers := router.Group("/servers/:serverId", authMiddleware)
	sites := servers.Group("/sites")

	registerSiteRoutes(sites, siteHandler)
	registerDeploymentRoutes(sites, deploymentHandler)
	registerSSLRoutes(sites, sslHandler)
	registerQueueRoutes(sites, queueHandler)
	registerCommandRoutes(sites, commandHandler)
	registerRedirectRoutes(sites, redirectHandler)
	registerFileRoutes(sites, fileHandler)
}

// RegisterWebhookRoutes registers webhook routes (implements app.WebhookRegistrar)
// These routes don't require authentication - they use deploy tokens for auth
func (m *Module) RegisterWebhookRoutes(router fiber.Router) {
	deps := m.Deps()

	// Create minimal task runner deps (webhooks don't need file service features)
	taskRunnerDeps := &servertasks.TaskRunnerDeps{
		DB:         deps.DB,
		Queue:      deps.Queue,
		Dispatcher: deps.Dispatcher,
		Logger:     deps.Logger,
	}

	svc := m.createServices(taskRunnerDeps)
	webhookHandler := handlers.NewWebhookHandler(svc.Deployment())

	// Deployment webhook - triggered by git providers (GitHub, GitLab, Bitbucket)
	// URL: /deploy/:siteId/:token
	router.Post("/deploy/:siteId/:token", webhookHandler.DeployWebhook)
	router.Get("/deploy/:siteId/:token", webhookHandler.DeployWebhook) // Some providers use GET
}

// DomainRepository returns the domain repository for cross-module access
func (m *Module) DomainRepository() dnscontracts.DomainRepository {
	return m.domainRepo
}

// registerGlobalSiteRoutes registers site routes not nested under servers
func registerGlobalSiteRoutes(router fiber.Router, handler *handlers.SiteHandler) {
	// Domain verification
	router.Get("/verify-domain", handler.VerifyDomain)
}

// registerSiteRoutes registers site CRUD and settings routes
func registerSiteRoutes(router fiber.Router, handler *handlers.SiteHandler) {
	// CRUD
	router.Get("/", handler.List)
	router.Post("/", handler.Create)
	router.Get("/:id", handler.Show)
	router.Put("/:id", handler.Update)
	router.Delete("/:id", handler.Delete)

	// Site deletion summary
	router.Get("/:id/deletion-summary", handler.GetDeletionSummary)

	// Deploy token
	router.Post("/:id/deploy-token/regenerate", handler.RegenerateDeployToken)

	// Deployment settings
	router.Put("/:id/deployment-settings", handler.UpdateDeploymentSettings)

	// Site settings page
	router.Get("/:id/settings", handler.GetSettings)
}

// registerDeploymentRoutes registers deployment-related routes
func registerDeploymentRoutes(router fiber.Router, handler *handlers.DeploymentHandler) {
	// Deployments
	router.Post("/:id/deploy", handler.Deploy)
	router.Get("/:id/deployments", handler.ListDeployments)
	router.Get("/:id/deployments/:deploymentId", handler.ShowDeployment)
	router.Post("/:id/rollback/:deploymentId", handler.Rollback)
	router.Delete("/:id/deployments/queued", handler.CancelQueuedDeployments)

	// Auto-deployment
	router.Post("/:id/auto-deployment/enable", handler.EnableAutoDeployment)
	router.Post("/:id/auto-deployment/disable", handler.DisableAutoDeployment)
}

// registerSSLRoutes registers SSL/TLS routes
func registerSSLRoutes(router fiber.Router, handler *handlers.SSLHandler) {
	router.Put("/:id/ssl", handler.UpdateSSL)
	router.Get("/:id/certificates", handler.ListCertificates)
}

// registerQueueRoutes registers queue routes
func registerQueueRoutes(router fiber.Router, handler *handlers.QueueHandler) {
	router.Get("/:id/queues", handler.ListQueues)
	router.Post("/:id/queues", handler.CreateQueue)
	router.Post("/:id/queues/sync", handler.SyncQueues)
	router.Delete("/:id/queues/:queueId", handler.DeleteQueue)

	// Auto-restart queue
	router.Post("/:id/auto-restart-queue/enable", handler.EnableAutoRestartQueue)
	router.Post("/:id/auto-restart-queue/disable", handler.DisableAutoRestartQueue)
}

// registerCommandRoutes registers command routes
func registerCommandRoutes(router fiber.Router, handler *handlers.CommandHandler) {
	router.Get("/:id/commands", handler.ListCommands)
	router.Post("/:id/commands", handler.CreateCommand)
	router.Delete("/:id/commands/:commandId", handler.DeleteCommand)
}

// registerRedirectRoutes registers redirect routes
func registerRedirectRoutes(router fiber.Router, handler *handlers.RedirectHandler) {
	router.Get("/:id/redirects", handler.ListRedirects)
	router.Post("/:id/redirects", handler.CreateRedirect)
	router.Delete("/:id/redirects/:redirectId", handler.DeleteRedirect)
}

// registerFileRoutes registers file management routes
func registerFileRoutes(router fiber.Router, handler *handlers.FileHandler) {
	router.Get("/:id/files", handler.ListFiles)
	router.Get("/:id/files/:file", handler.ShowFile)     // Get file content by encoded param
	router.Put("/:id/files/:file", handler.UpdateFile)   // Update file content by encoded param
	router.Patch("/:id/files/:file", handler.UpdateFile) // Update file content by encoded param (PATCH)
	router.Get("/:id/logs", handler.ListLogs)
}
