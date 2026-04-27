package git

import (
	gofiber "github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/middleware"
	"github.com/kkz6/launch-go/internal/modules/git/dto"
	"github.com/kkz6/launch-go/internal/modules/git/handlers"
	"github.com/kkz6/launch-go/internal/modules/git/services"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// RegisterRoutes registers all git routes. Standard CRUD on
// /source-controls maps to framework helpers; provider-discriminated
// reads (`:provider`-scoped installation lookups, OAuth callbacks) live
// in the SourceControlHandler.
func (m *Module) RegisterRoutes(router gofiber.Router, authMiddleware gofiber.Handler) {
	svc := m.createServices()
	handler := handlers.NewSourceControlHandler(svc.SourceControl())

	// Settings UI routes.
	settings := router.Group("/settings", authMiddleware, middleware.TeamScope(), middleware.VerifySubscription())
	settings.Get("/git-providers", handler.GetInstallationsWithCounts)
	settings.Get("/git-providers/:provider/installation-url", handler.GetInstallationURL)
	settings.Get("/git-providers/:provider/installations", handler.GetInstallations)
	settings.Get("/git-providers/:provider/installations/:installationId/repositories", handler.GetInstallationRepositories)
	settings.Get("/git-providers/:provider/installations/:installationId/cached-repositories", handler.GetCachedInstallationRepositories)
	settings.Post("/git-providers/:provider/installations/:installationId/refresh-repositories", handler.RefreshInstallationRepositories)
	settings.Get("/git-providers/:provider/callback", handler.HandleInstallationCallback)

	// App-based integration routes.
	integrations := router.Group("/integrations/git-apps", authMiddleware, middleware.TeamScope(), middleware.VerifySubscription())
	integrations.Get("/:provider/installation-url", handler.GetInstallationURL)
	integrations.Get("/:provider/installations", handler.GetInstallations)
	integrations.Get("/:provider/installations/:installationId", handler.GetInstallation)
	integrations.Get("/:provider/installations/:installationId/repositories", handler.GetInstallationRepositories)
	integrations.Get("/:provider/test-connection", handler.TestConnection)

	// Source-control CRUD via framework helpers.
	registerSourceControlRoutes(router.Group("/source-controls", authMiddleware, middleware.TeamScope(), middleware.VerifySubscription()), svc)
}

// RegisterWebhookRoutes registers webhook routes at the root level (no
// auth, no /api prefix). These routes receive callbacks from external
// git providers (GitHub, GitLab, Bitbucket).
func (m *Module) RegisterWebhookRoutes(router gofiber.Router) {
	deps := m.Deps()
	svc := m.createServices()
	webhookHandler := handlers.NewWebhookHandler(svc.SourceControl(), m.providerFactory, deps.Logger)
	webhookHandler.SetQueueClient(deps.Queue)

	router.Group("/webhooks/git").Post("/:provider", webhookHandler.HandleWebhook)
}

// RegisterAPIRoutes registers API-only routes.
func (m *Module) RegisterAPIRoutes(router gofiber.Router, authMiddleware gofiber.Handler) {
	svc := m.createServices()
	handler := handlers.NewSourceControlHandler(svc.SourceControl())

	api := router.Group("/git", authMiddleware, middleware.TeamScope(), middleware.VerifySubscription())
	registerSourceControlRoutes(api.Group("/source-controls"), svc)

	providers := api.Group("/providers")
	providers.Get("/:provider/installation-url", handler.GetInstallationURL)
	providers.Get("/:provider/installations", handler.GetInstallations)
	providers.Get("/:provider/installations/:installationId", handler.GetInstallation)
	providers.Get("/:provider/installations/:installationId/repositories", handler.GetInstallationRepositories)
	providers.Get("/:provider/test-connection", handler.TestConnection)
	providers.Post("/:provider/installations/:installationId/sync", handler.RefreshInstallationRepositories)
}

// registerSourceControlRoutes wires standard team-scoped CRUD on a
// /source-controls group via the framework route helpers.
func registerSourceControlRoutes(g gofiber.Router, svc *services.ServiceRegistry) {
	g.Get("/", fiberutil.Index("Source controls retrieved", svc.SourceControl().ListSourceControls))
	g.Get("/:id", fiberutil.Show("Source control retrieved", svc.SourceControl().GetSourceControl))
	g.Get("/:id/repositories", fiberutil.IndexNested("id", "Repositories retrieved", svc.SourceControl().GetSourceControlRepositories))
	g.Post("/", fiberutil.Create[dto.ConnectProviderRequest]("Provider connected successfully", svc.SourceControl().Connect))
	g.Delete("/:id", fiberutil.Delete(svc.SourceControl().Disconnect))
}
