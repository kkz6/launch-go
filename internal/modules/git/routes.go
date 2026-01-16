package git

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/git/handlers"
)

// RegisterRoutes registers all git routes
func (m *Module) RegisterRoutes(router fiber.Router, authMiddleware fiber.Handler) {
	deps := m.Deps()

	// Create services
	svc := m.createServices()

	// Create handlers
	handler := handlers.NewSourceControlHandler(svc.SourceControl())
	webhookHandler := handlers.NewWebhookHandler(svc.SourceControl(), m.providerFactory, deps.Logger)

	// Settings routes (authenticated)
	settings := router.Group("/settings", authMiddleware)
	{
		// Git providers page
		settings.Get("/git-providers", handler.GetInstallationsWithCounts)

		// Installation URL
		settings.Get("/git-providers/:provider/installation-url", handler.GetInstallationURL)

		// Get all installations for a provider
		settings.Get("/git-providers/:provider/installations", handler.GetInstallations)

		// Get repositories from API
		settings.Get("/git-providers/:provider/installations/:installationId/repositories", handler.GetInstallationRepositories)

		// Get cached repositories
		settings.Get("/git-providers/:provider/installations/:installationId/cached-repositories", handler.GetCachedInstallationRepositories)

		// Refresh repositories
		settings.Post("/git-providers/:provider/installations/:installationId/refresh-repositories", handler.RefreshInstallationRepositories)

		// Installation callback
		settings.Get("/git-providers/:provider/callback", handler.HandleInstallationCallback)
	}

	// App-based routes (authenticated)
	integrations := router.Group("/integrations/git-apps", authMiddleware)
	{
		// Get installation URL
		integrations.Get("/:provider/installation-url", handler.GetInstallationURL)

		// Get all installations
		integrations.Get("/:provider/installations", handler.GetInstallations)

		// Get single installation
		integrations.Get("/:provider/installations/:installationId", handler.GetInstallation)

		// Get installation repositories
		integrations.Get("/:provider/installations/:installationId/repositories", handler.GetInstallationRepositories)

		// Test connection
		integrations.Get("/:provider/test-connection", handler.TestConnection)
	}

	// Source controls CRUD (authenticated)
	sourceControls := router.Group("/source-controls", authMiddleware)
	{
		sourceControls.Get("/", handler.ListSourceControls)
		sourceControls.Get("/:id", handler.GetSourceControl)
		sourceControls.Get("/:id/repositories", handler.GetSourceControlRepositories)
		sourceControls.Post("/", handler.Connect)
		sourceControls.Delete("/:id", handler.Disconnect)
	}

	// Webhook routes (no auth required)
	webhooks := router.Group("/webhooks/git")
	{
		webhooks.Post("/:provider", webhookHandler.HandleWebhook)
	}
}

// RegisterAPIRoutes registers API-only routes
func (m *Module) RegisterAPIRoutes(router fiber.Router, authMiddleware fiber.Handler) {
	// Create services
	svc := m.createServices()

	// Create handler
	handler := handlers.NewSourceControlHandler(svc.SourceControl())

	// API v1 routes
	api := router.Group("/git", authMiddleware)
	{
		// Source controls
		api.Get("/source-controls", handler.ListSourceControls)
		api.Get("/source-controls/:id", handler.GetSourceControl)
		api.Get("/source-controls/:id/repositories", handler.GetSourceControlRepositories)
		api.Post("/source-controls", handler.Connect)
		api.Delete("/source-controls/:id", handler.Disconnect)

		// Providers
		api.Get("/providers/:provider/installation-url", handler.GetInstallationURL)
		api.Get("/providers/:provider/installations", handler.GetInstallations)
		api.Get("/providers/:provider/installations/:installationId", handler.GetInstallation)
		api.Get("/providers/:provider/installations/:installationId/repositories", handler.GetInstallationRepositories)
		api.Get("/providers/:provider/test-connection", handler.TestConnection)

		// Sync
		api.Post("/providers/:provider/installations/:installationId/sync", handler.RefreshInstallationRepositories)
	}
}
