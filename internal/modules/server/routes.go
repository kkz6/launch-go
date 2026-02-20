package server

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/middleware"
	"github.com/kkz6/launch-go/internal/modules/server/handlers"
	"github.com/kkz6/launch-go/internal/modules/server/services"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/pkg/signedurl"
)

// RegisterRoutes registers all server module routes
func (m *Module) RegisterRoutes(router fiber.Router, authMiddleware fiber.Handler, siteCounter handlers.SiteCounter) {
	deps := m.Deps()
	taskRunnerDeps := &tasks.TaskRunnerDeps{
		DB:          deps.DB,
		Queue:       deps.Queue,
		Dispatcher:  deps.Dispatcher,
		Logger:      deps.Logger,
		Broadcaster: deps.WebSocket,
		Notifier:    deps.Notifier,
	}
	handler := handlers.NewHandler(m.service, taskRunnerDeps, siteCounter, m.repos.LoadBalancerUpstream())

	// Create LB service and handler
	lbService := services.NewLoadBalancerService(deps.ServiceDeps(), m.repos, m.siteReader)
	lbHandler := handlers.NewLoadBalancerHandler(lbService)

	m.registerServerProviderRoutes(router, authMiddleware, handler)
	m.registerServerRoutes(router, authMiddleware, handler, lbHandler)
	m.registerSSHKeyRoutes(router, authMiddleware, handler)
}

// registerServerProviderRoutes registers server provider routes
func (m *Module) registerServerProviderRoutes(router fiber.Router, authMiddleware fiber.Handler, handler *handlers.Handler) {
	providers := router.Group("/server-providers", authMiddleware, middleware.TeamScope(), middleware.VerifySubscription())
	{
		providers.Get("/", handler.ListServerProviders)
	}
}

// registerServerRoutes registers all server-related routes
func (m *Module) registerServerRoutes(router fiber.Router, authMiddleware fiber.Handler, handler *handlers.Handler, lbHandler *handlers.LoadBalancerHandler) {
	servers := router.Group("/servers", authMiddleware, middleware.TeamScope(), middleware.VerifySubscription())
	{
		// Options (must be before /:id routes)
		servers.Get("/create-options", handler.GetCreateOptions)
		servers.Get("/archived", handler.ListArchived)

		// CRUD
		servers.Get("/", handler.List)
		servers.Post("/", handler.Create)
		servers.Get("/:id", handler.Show)
		servers.Get("/:id/page", handler.ShowPage)
		servers.Get("/:id/site-count", handler.GetSiteCount)
		servers.Delete("/:id", handler.Delete)

		// Provisioning actions (don't require provisioned server)
		servers.Post("/:id/retry-provision", handler.RetryProvision)
		servers.Get("/:id/provision-status", handler.GetProvisionStatus)
		servers.Get("/:id/provision-script-content", handler.GetProvisionScriptContent)
		servers.Post("/:id/archive", handler.Archive)
		servers.Post("/:id/unarchive", handler.Unarchive)

		// Tasks (needed during provisioning to show progress)
		servers.Get("/:id/tasks", handler.ListTasks)
		servers.Get("/:id/tasks/latest", handler.GetLatestTask)
		servers.Get("/:id/tasks/:taskId", handler.GetTask)

		// Routes that require a provisioned (running) server
		provisioned := middleware.RequireProvisionedServer()

		servers.Put("/:id", provisioned, handler.Update)
		servers.Patch("/:id", provisioned, handler.Update)

		// Actions
		servers.Post("/:id/reboot", provisioned, handler.Reboot)
		servers.Post("/:id/connect", provisioned, handler.Connect)
		servers.Post("/:id/vulnerability-audit", provisioned, handler.RunVulnerabilityAudit)

		// Services
		servers.Get("/:id/services", provisioned, handler.ListServices)
		servers.Get("/:id/services/create", provisioned, handler.GetAvailableServices)
		servers.Post("/:id/services", provisioned, handler.InstallService)
		servers.Post("/:id/services/:serviceId/:action", provisioned, handler.ServiceOperationByAction)
		servers.Post("/:id/services/:serviceId", provisioned, handler.ServiceOperation)

		// PHP
		servers.Get("/:id/php", provisioned, handler.ListPhpVersions)
		servers.Get("/:id/php-versions", provisioned, handler.ListInstalledPhpVersions)
		servers.Get("/:id/php/opcache/defaults", provisioned, handler.GetOpcacheDefaults)
		servers.Get("/:id/php/:phpId/opcache/status", provisioned, handler.GetOpcacheStatus)
		servers.Post("/:id/php/:phpId/opcache/reset", provisioned, handler.ResetOpcache)
		servers.Post("/:id/php/:phpId/opcache/configure", provisioned, handler.ConfigureOpcache)
		servers.Post("/:id/php/:phpId/extensions", provisioned, handler.InstallPhpExtension)
		servers.Delete("/:id/php/:phpId/extensions/:extension", provisioned, handler.UninstallPhpExtension)

		// Composer Packages
		servers.Get("/:id/packages", provisioned, handler.GetComposerAuth)
		servers.Put("/:id/packages", provisioned, handler.UpdateComposerAuth)

		// Firewall Rules
		servers.Get("/:id/firewall-rules", provisioned, handler.ListFirewallRules)
		servers.Post("/:id/firewall-rules", provisioned, handler.CreateFirewallRule)
		servers.Put("/:id/firewall-rules/:ruleId", provisioned, handler.UpdateFirewallRule)
		servers.Delete("/:id/firewall-rules/:ruleId", provisioned, handler.DeleteFirewallRule)

		// Cron Jobs
		servers.Get("/:id/crons", provisioned, handler.ListCrons)
		servers.Post("/:id/crons", provisioned, handler.CreateCron)
		servers.Put("/:id/crons/:cronId", provisioned, handler.UpdateCron)
		servers.Delete("/:id/crons/:cronId", provisioned, handler.DeleteCron)

		// Daemons
		servers.Get("/:id/daemons", provisioned, handler.ListDaemons)
		servers.Post("/:id/daemons", provisioned, handler.CreateDaemon)
		servers.Post("/:id/daemons/sync", provisioned, handler.SyncDaemons)
		servers.Put("/:id/daemons/:daemonId", provisioned, handler.UpdateDaemon)
		servers.Post("/:id/daemons/:daemonId/restart", provisioned, handler.RestartDaemon)
		servers.Delete("/:id/daemons/:daemonId", provisioned, handler.DeleteDaemon)

		// SSH Keys (server-specific)
		servers.Get("/:id/ssh-keys", provisioned, handler.ListServerSSHKeys)
		servers.Post("/:id/ssh-keys", provisioned, handler.AttachSSHKey)
		servers.Delete("/:id/ssh-keys/:sshKeyId", provisioned, handler.DetachSSHKey)

		// Metrics
		servers.Get("/:id/metrics", provisioned, handler.GetMetrics)
		servers.Get("/:id/metrics/latest", provisioned, handler.GetLatestMetric)

		// Logs
		servers.Get("/:id/logs", provisioned, handler.ListLogs)
		servers.Get("/:id/logs/:log", provisioned, handler.GetLogContent)

		// Load Balancer Upstreams
		servers.Get("/:id/upstreams/check-domain", provisioned, lbHandler.CheckDomain)
		servers.Get("/:id/upstreams", provisioned, lbHandler.ListUpstreams)
		servers.Post("/:id/upstreams", provisioned, lbHandler.CreateUpstream)
		servers.Get("/:id/upstreams/:upstreamId", provisioned, lbHandler.ShowUpstream)
		servers.Put("/:id/upstreams/:upstreamId", provisioned, lbHandler.UpdateUpstream)
		servers.Delete("/:id/upstreams/:upstreamId", provisioned, lbHandler.DeleteUpstream)

		// Load Balancer Backends
		servers.Get("/:id/upstreams/:upstreamId/backends", provisioned, lbHandler.ListBackends)
		servers.Post("/:id/upstreams/:upstreamId/backends", provisioned, lbHandler.AddBackend)
		servers.Put("/:id/upstreams/:upstreamId/backends/:backendId", provisioned, lbHandler.UpdateBackend)
		servers.Delete("/:id/upstreams/:upstreamId/backends/:backendId", provisioned, lbHandler.RemoveBackend)
		servers.Post("/:id/upstreams/:upstreamId/backends/:backendId/toggle-down", provisioned, lbHandler.ToggleBackendDown)

		// Load Balancer Health Checks
		servers.Get("/:id/upstreams/:upstreamId/health", provisioned, lbHandler.GetUpstreamHealth)
		servers.Post("/:id/upstreams/:upstreamId/health-check", provisioned, lbHandler.TriggerHealthCheck)
	}
}

// registerSSHKeyRoutes registers global SSH key routes
func (m *Module) registerSSHKeyRoutes(router fiber.Router, authMiddleware fiber.Handler, handler *handlers.Handler) {
	sshKeys := router.Group("/ssh-keys", authMiddleware, middleware.TeamScope(), middleware.VerifySubscription())
	{
		sshKeys.Get("/", handler.ListSSHKeys)
		sshKeys.Post("/", handler.CreateSSHKey)
		sshKeys.Post("/generate", handler.GenerateSSHKey)
		sshKeys.Delete("/:sshKeyId", handler.DeleteSSHKey)
	}
}

// RegisterWebhookRoutes registers all webhook routes (no auth required, uses signed URLs)
func (m *Module) RegisterWebhookRoutes(router fiber.Router) {
	provisionScriptHandler := handlers.NewProvisionScriptHandler(m.repos, m.service)
	metricsWebhookHandler := handlers.NewMetricsWebhookHandler(m.Deps().DB, m.Deps().Config.App.Key, m.Deps().WebSocket, m.Deps().Logger)

	m.registerProvisionScriptRoutes(router, provisionScriptHandler)
	m.registerMetricsWebhookRoutes(router, metricsWebhookHandler)
}

// registerProvisionScriptRoutes registers provision script routes (signed URL protected)
func (m *Module) registerProvisionScriptRoutes(router fiber.Router, handler *handlers.ProvisionScriptHandler) {
	router.Get("/servers/:id/provision-script",
		signedurl.RequireSignedURL(nil),
		handler.GetProvisionScript,
	)
}

// registerMetricsWebhookRoutes registers metrics/pulse webhook routes (signed URL protected)
func (m *Module) registerMetricsWebhookRoutes(router fiber.Router, handler *handlers.MetricsWebhookHandler) {
	router.Post("/webhooks/servers/:id/pulse",
		signedurl.RequireSignedURL(nil),
		handler.ReceivePulse,
	)
}
