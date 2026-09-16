package server

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/middleware"
	"github.com/kkz6/launch-go/internal/modules/server/dto"
	"github.com/kkz6/launch-go/internal/modules/server/handlers"
	"github.com/kkz6/launch-go/internal/modules/server/services"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
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
	m.registerServerRoutes(router, authMiddleware, handler, lbHandler, lbService)
	m.registerSSHKeyRoutes(router, authMiddleware, handler)
}

// registerServerProviderRoutes registers server provider routes
func (m *Module) registerServerProviderRoutes(router fiber.Router, authMiddleware fiber.Handler, _ *handlers.Handler) {
	providers := router.Group("/server-providers", authMiddleware, middleware.TeamScope(), middleware.VerifySubscription())
	{
		providers.Get("/", fiberutil.Index("Server providers retrieved", m.service.ListServerProviders))
		providers.Post("/", middleware.Can("server_provider.create"), fiberutil.Create[dto.CreateServerProviderRequest]("Server provider connected", m.service.CreateServerProvider))
		providers.Delete("/:id", middleware.Can("server_provider.delete"), fiberutil.Delete(m.service.DeleteServerProvider))
	}
}

// registerServerRoutes registers all server-related routes
func (m *Module) registerServerRoutes(router fiber.Router, authMiddleware fiber.Handler, handler *handlers.Handler, lbHandler *handlers.LoadBalancerHandler, lbService *services.LoadBalancerService) {
	servers := router.Group("/servers", authMiddleware, middleware.TeamScope(), middleware.VerifySubscription())
	{
		// Options (must be before /:id routes)
		servers.Get("/create-options", handler.GetCreateOptions)
		servers.Get("/archived", fiberutil.Index("Archived servers retrieved", m.service.ListArchivedServers))

		// CRUD
		servers.Get("/", fiberutil.Index("Servers retrieved", m.service.ListServers))
		servers.Post("/", middleware.Can("server.create"), fiberutil.Create[dto.CreateServerRequest]("Server created", m.service.CreateServer))
		servers.Get("/:id", fiberutil.Show("Server retrieved", m.service.GetServer))
		servers.Get("/:id/page", fiberutil.Show("Server page data retrieved", m.service.GetShowPageData))
		servers.Get("/:id/site-count", handler.GetSiteCount)
		servers.Delete("/:id", middleware.Can("server.delete"), fiberutil.Delete(m.service.DeleteServer))

		// Provisioning actions (don't require provisioned server)
		servers.Post("/:id/retry-provision", middleware.Can("server.manage"), fiberutil.Action("Server provisioning has been queued", m.service.RetryProvision))
		// Custom-server only: user clicks this after pasting the provision
		// script into their box. One synchronous SSH attempt; on success
		// the server advances to provisioning.
		servers.Post("/:id/try-connection", middleware.Can("server.manage"), fiberutil.Action("Connected — provisioning started", m.service.TryConnection))
		servers.Get("/:id/provision-status", handler.GetProvisionStatus)
		servers.Get("/:id/provision-script-content", handler.GetProvisionScriptContent)
		// Manually re-detect OS facts on an already-provisioned server.
		// Useful for backfilling rows that pre-date the detect_os
		// provision step, or for refreshing after a kernel/distro
		// upgrade. The endpoint enqueues a job; the actual SSH happens
		// in the worker and the result is broadcast via server.updated.
		servers.Post("/:id/detect-os", middleware.Can("server.manage"), fiberutil.Action("OS detection has been queued", m.service.BackfillDetectedOS))
		servers.Post("/:id/archive", middleware.Can("server.manage"), fiberutil.Action("Server archived", m.service.ArchiveServer))
		servers.Post("/:id/unarchive", middleware.Can("server.manage"), fiberutil.Action("Server unarchived", m.service.UnarchiveServer))

		// Tasks (needed during provisioning to show progress)
		servers.Get("/:id/tasks", handler.ListTasks)
		servers.Get("/:id/tasks/latest", fiberutil.IndexNested("id", "Latest task retrieved", m.service.GetLatestTask))
		servers.Get("/:id/tasks/:taskId", fiberutil.ShowNested("id", "taskId", "Task retrieved", m.service.GetTask))

		// Routes that require a provisioned (running) server
		provisioned := middleware.RequireProvisionedServer()

		updateServer := fiberutil.Update[dto.UpdateServerRequest]("Server updated", m.service.UpdateServer)
		servers.Put("/:id", provisioned, middleware.Can("server.update"), updateServer)
		servers.Patch("/:id", provisioned, middleware.Can("server.update"), updateServer)

		// Actions
		servers.Post("/:id/reboot", provisioned, middleware.Can("server.reboot"), fiberutil.Action("Server reboot initiated", m.service.RebootServer))
		servers.Post("/:id/connect", provisioned, middleware.Can("server.manage"), fiberutil.Action("Server connection successful", m.service.ConnectServer))
		servers.Post("/:id/vulnerability-audit", provisioned, middleware.Can("server.audit"), fiberutil.Bind(handler.RunVulnerabilityAudit))

		// Services
		servers.Get("/:id/services", provisioned, fiberutil.IndexNested("id", "Services retrieved", m.service.ListServices))
		// Launch Agent update-availability check — drives the update banner.
		servers.Get("/:id/agent-version", provisioned, fiberutil.IndexNested("id", "Agent version retrieved", m.service.GetAgentVersionInfo))
		servers.Get("/:id/services/create", provisioned, fiberutil.IndexNested("id", "Available services retrieved", m.service.GetAvailableServices))
		servers.Post("/:id/services", provisioned, middleware.Can("server.manage"), fiberutil.CreateNested[dto.CreateServiceRequest]("id", "Service installation initiated", m.service.InstallService))
		servers.Post("/:id/services/:serviceId/:action", provisioned, middleware.Can("server.manage"), handler.ServiceOperationByAction)
		servers.Post("/:id/services/:serviceId", provisioned, middleware.Can("server.manage"), fiberutil.Validate(handler.ServiceOperation))

		// PHP
		servers.Get("/:id/php", provisioned, fiberutil.IndexNested("id", "PHP versions retrieved", m.service.GetPhpVersions))
		servers.Get("/:id/php-versions", provisioned, fiberutil.IndexNested("id", "Installed PHP versions retrieved", m.service.GetInstalledPhpVersions))
		servers.Get("/:id/php/opcache/defaults", provisioned, handler.GetOpcacheDefaults)
		servers.Get("/:id/php/:phpId/opcache/status", provisioned, fiberutil.ShowNested("id", "phpId", "OPcache status retrieved", m.service.GetOpcacheStatus))
		servers.Post("/:id/php/:phpId/default", provisioned, middleware.Can("server.manage"), fiberutil.ActionItemNested("id", "phpId", "Default PHP version update initiated", m.service.SetDefaultPhpVersion))
		servers.Post("/:id/php/:phpId/patch", provisioned, middleware.Can("server.manage"), fiberutil.ActionItemNested("id", "phpId", "PHP patch initiated", m.service.PatchPhpVersion))
		servers.Post("/:id/php/:phpId/opcache/reset", provisioned, middleware.Can("server.manage"), fiberutil.ActionItemNested("id", "phpId", "OPcache reset initiated", m.service.ResetOpcache))
		servers.Post("/:id/php/:phpId/opcache/configure", provisioned, middleware.Can("server.manage"), fiberutil.Validate(handler.ConfigureOpcache))
		servers.Post("/:id/php/:phpId/extensions", provisioned, middleware.Can("server.manage"), fiberutil.Validate(handler.InstallPhpExtension))
		servers.Delete("/:id/php/:phpId/extensions/:extension", provisioned, middleware.Can("server.manage"), handler.UninstallPhpExtension)
		servers.Get("/:id/php/:phpId/configuration/:kind", provisioned, handler.GetPHPConfiguration)
		servers.Put("/:id/php/:phpId/configuration/:kind", provisioned, middleware.Can("server.manage"), fiberutil.Validate(handler.UpdatePHPConfiguration))

		// Composer Packages
		servers.Get("/:id/packages", provisioned, fiberutil.IndexNested("id", "Composer auth retrieved", m.service.GetComposerAuth))
		servers.Put("/:id/packages", provisioned, middleware.Can("server.manage"), fiberutil.Validate(handler.UpdateComposerAuth))

		// Firewall Rules
		servers.Get("/:id/firewall-rules", provisioned, fiberutil.IndexNested("id", "Firewall rules retrieved", m.service.ListFirewallRules))
		servers.Post("/:id/firewall-rules", provisioned, middleware.Can("server.manage"), fiberutil.CreateNested[dto.CreateFirewallRuleRequest]("id", "Firewall rule created", m.service.CreateFirewallRule))
		servers.Put("/:id/firewall-rules/:ruleId", provisioned, middleware.Can("server.manage"), fiberutil.UpdateNested[dto.UpdateFirewallRuleRequest]("id", "ruleId", "Firewall rule updated", m.service.UpdateFirewallRule))
		servers.Delete("/:id/firewall-rules/:ruleId", provisioned, middleware.Can("server.manage"), fiberutil.DeleteNested("id", "ruleId", m.service.DeleteFirewallRule))

		// Cron Jobs
		servers.Get("/:id/crons", provisioned, fiberutil.IndexNested("id", "Cron jobs retrieved", m.service.ListCrons))
		servers.Post("/:id/crons", provisioned, middleware.Can("server.manage"), fiberutil.CreateNested[dto.CreateCronRequest]("id", "Cron job created", m.service.CreateCron))
		servers.Put("/:id/crons/:cronId", provisioned, middleware.Can("server.manage"), fiberutil.UpdateNested[dto.UpdateCronRequest]("id", "cronId", "Cron job updated", m.service.UpdateCron))
		servers.Delete("/:id/crons/:cronId", provisioned, middleware.Can("server.manage"), fiberutil.DeleteNested("id", "cronId", m.service.DeleteCron))

		// Daemons
		servers.Get("/:id/daemons", provisioned, fiberutil.IndexNested("id", "Daemons retrieved", m.service.ListDaemons))
		servers.Post("/:id/daemons", provisioned, middleware.Can("server.manage"), fiberutil.CreateNested[dto.CreateDaemonRequest]("id", "Daemon created", m.service.CreateDaemon))
		servers.Post("/:id/daemons/sync", provisioned, middleware.Can("server.manage"), fiberutil.ActionNested("id", "Daemon sync initiated", m.service.SyncDaemonsStatus))
		servers.Patch("/:id/daemons/:daemonId", provisioned, middleware.Can("server.manage"), fiberutil.UpdateNested[dto.UpdateDaemonRequest]("id", "daemonId", "Daemon updated", m.service.UpdateDaemon))
		servers.Post("/:id/daemons/:daemonId/restart", provisioned, middleware.Can("server.manage"), fiberutil.ActionItemNested("id", "daemonId", "Daemon restart initiated", m.service.RestartDaemon))
		servers.Delete("/:id/daemons/:daemonId", provisioned, middleware.Can("server.manage"), fiberutil.DeleteNested("id", "daemonId", m.service.DeleteDaemon))

		// SSH Keys (server-specific)
		servers.Get("/:id/ssh-keys", provisioned, fiberutil.IndexNested("id", "SSH keys retrieved", m.service.ListServerSSHKeys))
		servers.Post("/:id/ssh-keys", provisioned, middleware.Can("server.manage"), fiberutil.Validate(handler.AttachSSHKey))
		servers.Delete("/:id/ssh-keys/:sshKeyId", provisioned, middleware.Can("server.manage"), fiberutil.DeleteNested("id", "sshKeyId", m.service.DetachSSHKey))

		// Metrics
		servers.Get("/:id/metrics", provisioned, handler.GetMetrics)
		servers.Get("/:id/metrics/latest", provisioned, fiberutil.IndexNested("id", "Latest metric retrieved", m.service.GetLatestMetric))

		// Logs
		servers.Get("/:id/logs", provisioned, handler.ListLogs)
		servers.Get("/:id/logs/:log", provisioned, handler.GetLogContent)

		// Load Balancer Upstreams
		servers.Get("/:id/upstreams/check-domain", provisioned, lbHandler.CheckDomain)
		servers.Get("/:id/upstreams", provisioned, fiberutil.IndexNested("id", "Upstreams retrieved", lbService.Upstreams))
		servers.Post("/:id/upstreams", provisioned, middleware.Can("server.manage"), fiberutil.CreateNested[dto.CreateUpstreamRequest]("id", "Upstream created", lbService.UpstreamCreate))
		servers.Get("/:id/upstreams/:upstreamId", provisioned, fiberutil.ShowNested("id", "upstreamId", "Upstream retrieved", lbService.UpstreamShow))
		servers.Put("/:id/upstreams/:upstreamId", provisioned, middleware.Can("server.manage"), fiberutil.UpdateNested[dto.UpdateUpstreamRequest]("id", "upstreamId", "Upstream updated", lbService.UpstreamUpdate))
		servers.Delete("/:id/upstreams/:upstreamId", provisioned, middleware.Can("server.manage"), fiberutil.DeleteNested("id", "upstreamId", lbService.UpstreamDelete))

		// Load Balancer Backends (doubly-nested under server + upstream)
		servers.Get("/:id/upstreams/:upstreamId/backends", provisioned, fiberutil.IndexDoubleNested("id", "upstreamId", "Backends retrieved", lbService.Backends))
		servers.Post("/:id/upstreams/:upstreamId/backends", provisioned, middleware.Can("server.manage"), fiberutil.CreateDoubleNested[dto.AddBackendRequest]("id", "upstreamId", "Backend added", lbService.BackendCreate))
		servers.Put("/:id/upstreams/:upstreamId/backends/:backendId", provisioned, middleware.Can("server.manage"), fiberutil.UpdateDoubleNested[dto.UpdateBackendRequest]("id", "upstreamId", "backendId", "Backend updated", lbService.BackendUpdate))
		servers.Delete("/:id/upstreams/:upstreamId/backends/:backendId", provisioned, middleware.Can("server.manage"), fiberutil.DeleteDoubleNested("id", "upstreamId", "backendId", lbService.BackendDelete))
		servers.Post("/:id/upstreams/:upstreamId/backends/:backendId/toggle-down", provisioned, middleware.Can("server.manage"), lbHandler.ToggleBackendDown)

		// Load Balancer Health Checks
		servers.Get("/:id/upstreams/:upstreamId/health", provisioned, fiberutil.ShowNested("id", "upstreamId", "Upstream health retrieved", lbService.UpstreamHealth))
		servers.Post("/:id/upstreams/:upstreamId/health-check", provisioned, middleware.Can("server.manage"), fiberutil.ActionItemNested("id", "upstreamId", "Health check triggered", lbService.UpstreamHealthCheck))
	}
}

// registerSSHKeyRoutes registers global SSH key routes. The :id path
// param holds the SSH key id; the legacy :sshKeyId name is preserved on
// the nested server route only.
func (m *Module) registerSSHKeyRoutes(router fiber.Router, authMiddleware fiber.Handler, handler *handlers.Handler) {
	sshKeys := router.Group("/ssh-keys", authMiddleware, middleware.TeamScope(), middleware.VerifySubscription())
	{
		sshKeys.Get("/", handler.ListSSHKeys)
		sshKeys.Post("/", middleware.Can("server.manage"), fiberutil.Create[dto.CreateSSHKeyRequest]("SSH key created", m.service.CreateSSHKey))
		sshKeys.Post("/generate", middleware.Can("server.manage"), fiberutil.Validate(handler.GenerateSSHKey))
		sshKeys.Delete("/:id", middleware.Can("server.manage"), fiberutil.Delete(m.service.DeleteSSHKey))
	}
}

// RegisterWebhookRoutes registers all webhook routes (no auth required, uses signed URLs)
func (m *Module) RegisterWebhookRoutes(router fiber.Router) {
	provisionScriptHandler := handlers.NewProvisionScriptHandler(m.repos, m.service)
	metricsWebhookHandler := handlers.NewMetricsWebhookHandler(m.Deps().DB, m.Deps().Config.App.Key, m.Deps().WebSocket, m.Deps().Logger)

	m.registerProvisionScriptRoutes(router, provisionScriptHandler)
	m.registerMetricsWebhookRoutes(router, metricsWebhookHandler)
}

// registerProvisionScriptRoutes registers the provision script route. Lives
// at `/provision/:id` rather than `/servers/:id/provision-script` so it
// doesn't collide with the `/servers/*` auth group registered by
// registerServerRoutes — Fiber group middleware is a wildcard `Use` that
// matches any subpath, so a later root-level route under `/servers/...`
// still gets intercepted by the earlier auth middleware and returns 401.
func (m *Module) registerProvisionScriptRoutes(router fiber.Router, handler *handlers.ProvisionScriptHandler) {
	router.Get("/provision/:id",
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
