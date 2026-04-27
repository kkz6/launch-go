package site

import (
	"time"

	gofiber "github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/middleware"
	dnscontracts "github.com/kkz6/launch-go/internal/modules/dns/contracts"
	servertasks "github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/modules/site/dto"
	"github.com/kkz6/launch-go/internal/modules/site/handlers"
	"github.com/kkz6/launch-go/internal/modules/site/services"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// RegisterRoutes registers all site module routes. Sub-resources (queues,
// commands, redirects) are wired directly to the framework
// double-nested helpers; only routes with bespoke shapes (file editing,
// feature toggles, deployment composites) keep dedicated handlers.
func (m *Module) RegisterRoutes(router gofiber.Router, authMiddleware gofiber.Handler) {
	deps := m.Deps()

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
	h.SetDomainRepository(m.domainRepo)

	// Top-level site routes (not nested under servers).
	sitesGlobal := router.Group("/sites", authMiddleware, middleware.TeamScope(), middleware.VerifySubscription())
	m.registerGlobalSiteRoutes(sitesGlobal, h.Site)

	// Sites are nested under servers (require provisioned server).
	servers := router.Group("/servers/:serverId", authMiddleware, middleware.TeamScope(), middleware.VerifySubscription())
	sites := servers.Group("/sites", middleware.RequireProvisionedServer("serverId"))

	m.registerSiteRoutes(sites, h.Site, svc)
	m.registerDeploymentRoutes(sites, h.Deployment)
	m.registerSSLRoutes(sites, h.SSL)
	m.registerSSLListRoutes(sites, svc)
	m.registerQueueRoutes(sites, h.Queue, svc)
	m.registerCommandRoutes(sites, svc)
	m.registerRedirectRoutes(sites, svc)
	m.registerFileRoutes(sites, h.File)
	m.registerFeatureRoutes(sites, h.Feature)
}

// RegisterWebhookRoutes registers webhook routes (implements
// app.WebhookRegistrar). These routes don't require authentication —
// they use deploy tokens for auth.
func (m *Module) RegisterWebhookRoutes(router gofiber.Router) {
	deps := m.Deps()

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

	// Deployment webhook — triggered by git providers (GitHub, GitLab,
	// Bitbucket). URL: /deploy/:siteId/:token.
	rl := middleware.RateLimit(10, time.Minute)
	router.Post("/deploy/:siteId/:token", rl, h.Webhook.DeployWebhook)
	router.Get("/deploy/:siteId/:token", rl, h.Webhook.DeployWebhook)
}

// DomainRepository returns the domain repository for cross-module access.
func (m *Module) DomainRepository() dnscontracts.DomainRepository {
	return m.domainRepo
}

// registerGlobalSiteRoutes registers site routes not nested under servers.
func (m *Module) registerGlobalSiteRoutes(router gofiber.Router, handler *handlers.SiteHandler) {
	router.Get("/create-options", handler.GetCreateOptions)
	router.Get("/verify-domain", handler.VerifyDomain)
}

// registerSiteRoutes registers site CRUD and settings routes.
func (m *Module) registerSiteRoutes(router gofiber.Router, handler *handlers.SiteHandler, svc *services.ServiceRegistry) {
	site := svc.Site()

	// CRUD via single-nested helpers (sites are the leaf resource under
	// :serverId; the helper extracts :serverId as the parent).
	router.Get("/", fiberutil.IndexNested("serverId", "Sites retrieved", site.List))
	router.Post("/", fiberutil.CreateNested[dto.CreateSiteRequest]("serverId", "Site created", site.Create))
	router.Get("/:id", handler.Show) // bespoke: enriched with queue count + source-control info
	router.Put("/:id", fiberutil.UpdateNested[dto.UpdateSiteRequest]("serverId", "id", "Site updated", site.Update))
	router.Delete("/:id", handler.Delete) // bespoke: returns 200 + status message (async deletion)

	router.Get("/:id/deletion-resources", fiberutil.ShowNested("serverId", "id", "Deletion summary retrieved", site.GetDeletionSummary))
	router.Post("/:id/deploy-token/regenerate", handler.RegenerateDeployToken)
	router.Patch("/:id/deployment-settings", handler.UpdateDeploymentSettings)
	router.Get("/:id/settings", handler.GetSettings)
}

// registerDeploymentRoutes registers deployment-related routes.
func (m *Module) registerDeploymentRoutes(router gofiber.Router, handler *handlers.DeploymentHandler) {
	router.Post("/:id/deploy", handler.Deploy)
	router.Get("/:id/deployments", handler.ListDeployments)
	router.Get("/:id/deployments/:deploymentId", handler.ShowDeployment)
	router.Post("/:id/rollback/:deploymentId", handler.Rollback)
	router.Delete("/:id/deployments/queued", handler.CancelQueuedDeployments)

	router.Post("/:id/autodeploy", handler.ToggleAutoDeployment)
	router.Post("/:id/auto-deployment/enable", handler.EnableAutoDeployment)
	router.Post("/:id/auto-deployment/disable", handler.DisableAutoDeployment)
}

// registerSSLRoutes registers SSL/TLS routes.
func (m *Module) registerSSLRoutes(router gofiber.Router, handler *handlers.SSLHandler) {
	router.Put("/:id/ssl", handler.UpdateSSL)
}

// registerSSLListRoutes wires the certificates list via the
// double-nested helper. Split out so it can take the service registry.
func (m *Module) registerSSLListRoutes(router gofiber.Router, svc *services.ServiceRegistry) {
	router.Get("/:id/certificates", fiberutil.IndexDoubleNested("serverId", "id", "Certificates retrieved", svc.SSL().ListCertificates))
}

// registerQueueRoutes registers queue routes. CRUD goes through the
// double-nested helpers; the auto-restart toggle has a custom dynamic
// success message and stays on the bespoke handler.
func (m *Module) registerQueueRoutes(router gofiber.Router, handler *handlers.QueueHandler, svc *services.ServiceRegistry) {
	q := svc.Queue()
	router.Get("/:id/queues", fiberutil.IndexDoubleNested("serverId", "id", "Queues retrieved", q.List))
	router.Post("/:id/queues", fiberutil.CreateDoubleNested[dto.CreateQueueRequest]("serverId", "id", "Queue created", q.Create))
	router.Post("/:id/queues/sync", fiberutil.ActionDoubleNested("serverId", "id", "Queue sync initiated", q.SyncStatus))
	router.Patch("/:id/queues/:queueId", fiberutil.UpdateDoubleNested[dto.UpdateQueueRequest]("serverId", "id", "queueId", "Queue updated", q.Update))
	router.Post("/:id/queues/:queueId/restart", fiberutil.ActionItemDoubleNested("serverId", "id", "queueId", "Queue restart initiated", q.Restart))
	router.Delete("/:id/queues/:queueId", fiberutil.DeleteDoubleNested("serverId", "id", "queueId", q.Delete))

	router.Put("/:id/auto-restart-queue", handler.UpdateAutoRestartQueue)
}

// registerCommandRoutes registers command routes via double-nested helpers.
func (m *Module) registerCommandRoutes(router gofiber.Router, svc *services.ServiceRegistry) {
	c := svc.Command()
	router.Get("/:id/commands", fiberutil.IndexDoubleNested("serverId", "id", "Commands retrieved", c.List))
	router.Post("/:id/commands", fiberutil.CreateDoubleNested[dto.CreateCommandRequest]("serverId", "id", "Command created", c.Create))
	router.Delete("/:id/commands/:commandId", fiberutil.DeleteDoubleNested("serverId", "id", "commandId", c.Delete))
}

// registerRedirectRoutes registers redirect routes via double-nested helpers.
func (m *Module) registerRedirectRoutes(router gofiber.Router, svc *services.ServiceRegistry) {
	r := svc.Redirect()
	router.Get("/:id/redirects", fiberutil.IndexDoubleNested("serverId", "id", "Redirects retrieved", r.List))
	router.Post("/:id/redirects", fiberutil.CreateDoubleNested[dto.CreateRedirectRequest]("serverId", "id", "Redirect created", r.Create))
	router.Delete("/:id/redirects/:redirectId", fiberutil.DeleteDoubleNested("serverId", "id", "redirectId", r.Delete))
}

// registerFileRoutes registers file management routes.
func (m *Module) registerFileRoutes(router gofiber.Router, handler *handlers.FileHandler) {
	router.Get("/:id/files", handler.ListFiles)
	router.Get("/:id/files/:file", handler.ShowFile)
	router.Put("/:id/files/:file", handler.UpdateFile)
	router.Patch("/:id/files/:file", handler.UpdateFile)
	router.Get("/:id/logs", handler.ListLogs)
}

// registerFeatureRoutes registers Laravel feature management routes.
func (m *Module) registerFeatureRoutes(router gofiber.Router, handler *handlers.FeatureHandler) {
	router.Post("/:id/features/:feature/enable", handler.EnableFeature)
	router.Post("/:id/features/:feature/disable", handler.DisableFeature)
}
