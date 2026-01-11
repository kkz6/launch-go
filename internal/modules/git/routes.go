package git

import (
	"github.com/gofiber/fiber/v2"
)

// RegisterRoutes registers all git routes
func (m *Module) RegisterRoutes(router fiber.Router, authMiddleware fiber.Handler) {
	// Settings routes (authenticated)
	settings := router.Group("/settings", authMiddleware)
	{
		// Git providers page
		settings.Get("/git-providers", m.handler.GetInstallationsWithCounts)

		// Installation URL
		settings.Get("/git-providers/:provider/installation-url", m.handler.GetInstallationURL)

		// Get all installations for a provider
		settings.Get("/git-providers/:provider/installations", m.handler.GetInstallations)

		// Get repositories from API
		settings.Get("/git-providers/:provider/installations/:installationId/repositories", m.handler.GetInstallationRepositories)

		// Get cached repositories
		settings.Get("/git-providers/:provider/installations/:installationId/cached-repositories", m.handler.GetCachedInstallationRepositories)

		// Refresh repositories
		settings.Post("/git-providers/:provider/installations/:installationId/refresh-repositories", m.handler.RefreshInstallationRepositories)

		// Installation callback
		settings.Get("/git-providers/:provider/callback", m.handler.HandleInstallationCallback)
	}

	// App-based routes (authenticated)
	integrations := router.Group("/integrations/git-apps", authMiddleware)
	{
		// Get installation URL
		integrations.Get("/:provider/installation-url", m.handler.GetInstallationURL)

		// Get all installations
		integrations.Get("/:provider/installations", m.handler.GetInstallations)

		// Get single installation
		integrations.Get("/:provider/installations/:installationId", m.handler.GetInstallation)

		// Get installation repositories
		integrations.Get("/:provider/installations/:installationId/repositories", m.handler.GetInstallationRepositories)

		// Test connection
		integrations.Get("/:provider/test-connection", m.handler.TestConnection)
	}

	// Source controls CRUD (authenticated)
	sourceControls := router.Group("/source-controls", authMiddleware)
	{
		sourceControls.Get("/", m.handler.ListSourceControls)
		sourceControls.Get("/:id", m.handler.GetSourceControl)
		sourceControls.Post("/", m.handler.Connect)
		sourceControls.Delete("/:id", m.handler.Disconnect)
	}

	// Webhook routes (no auth required)
	webhooks := router.Group("/webhooks/git")
	{
		webhooks.Post("/:provider", m.webhookHandler.HandleWebhook)
	}
}

// RegisterAPIRoutes registers API-only routes
func (m *Module) RegisterAPIRoutes(router fiber.Router, authMiddleware fiber.Handler) {
	// API v1 routes
	api := router.Group("/git", authMiddleware)
	{
		// Source controls
		api.Get("/source-controls", m.handler.ListSourceControls)
		api.Get("/source-controls/:id", m.handler.GetSourceControl)
		api.Post("/source-controls", m.handler.Connect)
		api.Delete("/source-controls/:id", m.handler.Disconnect)

		// Providers
		api.Get("/providers/:provider/installation-url", m.handler.GetInstallationURL)
		api.Get("/providers/:provider/installations", m.handler.GetInstallations)
		api.Get("/providers/:provider/installations/:installationId", m.handler.GetInstallation)
		api.Get("/providers/:provider/installations/:installationId/repositories", m.handler.GetInstallationRepositories)
		api.Get("/providers/:provider/test-connection", m.handler.TestConnection)

		// Sync
		api.Post("/providers/:provider/installations/:installationId/sync", m.handler.RefreshInstallationRepositories)
	}
}
