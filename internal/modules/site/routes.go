package site

import (
	"github.com/gofiber/fiber/v2"
)

// RegisterRoutes registers all site module routes
func (m *Module) RegisterRoutes(router fiber.Router, authMiddleware fiber.Handler) {
	// Sites are nested under servers
	servers := router.Group("/servers/:serverId", authMiddleware)
	sites := servers.Group("/sites")

	m.registerSiteRoutes(sites)
	m.registerDeploymentRoutes(sites)
	m.registerSSLRoutes(sites)
	m.registerQueueRoutes(sites)
	m.registerCommandRoutes(sites)
	m.registerRedirectRoutes(sites)
	m.registerFileRoutes(sites)
}

// registerSiteRoutes registers site CRUD and settings routes
func (m *Module) registerSiteRoutes(router fiber.Router) {
	// CRUD
	router.Get("/", m.siteHandler.List)
	router.Post("/", m.siteHandler.Create)
	router.Get("/:id", m.siteHandler.Show)
	router.Put("/:id", m.siteHandler.Update)
	router.Delete("/:id", m.siteHandler.Delete)

	// Site deletion summary
	router.Get("/:id/deletion-summary", m.siteHandler.GetDeletionSummary)

	// Deploy token
	router.Post("/:id/deploy-token/regenerate", m.siteHandler.RegenerateDeployToken)

	// Deployment settings
	router.Put("/:id/deployment-settings", m.siteHandler.UpdateDeploymentSettings)

	// Site settings page
	router.Get("/:id/settings", m.siteHandler.GetSettings)
}

// registerDeploymentRoutes registers deployment-related routes
func (m *Module) registerDeploymentRoutes(router fiber.Router) {
	// Deployments
	router.Post("/:id/deploy", m.deploymentHandler.Deploy)
	router.Get("/:id/deployments", m.deploymentHandler.ListDeployments)
	router.Get("/:id/deployments/:deploymentId", m.deploymentHandler.ShowDeployment)
	router.Post("/:id/rollback/:deploymentId", m.deploymentHandler.Rollback)
	router.Delete("/:id/deployments/queued", m.deploymentHandler.CancelQueuedDeployments)

	// Auto-deployment
	router.Post("/:id/auto-deployment/enable", m.deploymentHandler.EnableAutoDeployment)
	router.Post("/:id/auto-deployment/disable", m.deploymentHandler.DisableAutoDeployment)
}

// registerSSLRoutes registers SSL/TLS routes
func (m *Module) registerSSLRoutes(router fiber.Router) {
	router.Put("/:id/ssl", m.sslHandler.UpdateSSL)
	router.Get("/:id/certificates", m.sslHandler.ListCertificates)
}

// registerQueueRoutes registers queue routes
func (m *Module) registerQueueRoutes(router fiber.Router) {
	router.Get("/:id/queues", m.queueHandler.ListQueues)
	router.Post("/:id/queues", m.queueHandler.CreateQueue)
	router.Post("/:id/queues/sync", m.queueHandler.SyncQueues)
	router.Delete("/:id/queues/:queueId", m.queueHandler.DeleteQueue)

	// Auto-restart queue
	router.Post("/:id/auto-restart-queue/enable", m.queueHandler.EnableAutoRestartQueue)
	router.Post("/:id/auto-restart-queue/disable", m.queueHandler.DisableAutoRestartQueue)
}

// registerCommandRoutes registers command routes
func (m *Module) registerCommandRoutes(router fiber.Router) {
	router.Get("/:id/commands", m.commandHandler.ListCommands)
	router.Post("/:id/commands", m.commandHandler.CreateCommand)
	router.Delete("/:id/commands/:commandId", m.commandHandler.DeleteCommand)
}

// registerRedirectRoutes registers redirect routes
func (m *Module) registerRedirectRoutes(router fiber.Router) {
	router.Get("/:id/redirects", m.redirectHandler.ListRedirects)
	router.Post("/:id/redirects", m.redirectHandler.CreateRedirect)
	router.Delete("/:id/redirects/:redirectId", m.redirectHandler.DeleteRedirect)
}

// registerFileRoutes registers file management routes
func (m *Module) registerFileRoutes(router fiber.Router) {
	router.Get("/:id/files", m.fileHandler.ListFiles)
	router.Get("/:id/files/:file", m.fileHandler.ShowFile)        // Get file content by encoded param
	router.Put("/:id/files/:file", m.fileHandler.UpdateFile)      // Update file content by encoded param
	router.Patch("/:id/files/:file", m.fileHandler.UpdateFile)    // Update file content by encoded param (PATCH)
	router.Get("/:id/logs", m.fileHandler.ListLogs)
}
