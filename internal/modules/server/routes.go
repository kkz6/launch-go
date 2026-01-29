package server

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/middleware"
	"github.com/kkz6/launch-go/internal/modules/server/handlers"
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
	}
	handler := handlers.NewHandler(m.service, taskRunnerDeps, siteCounter)

	m.registerServerProviderRoutes(router, authMiddleware, handler)
	m.registerServerRoutes(router, authMiddleware, handler)
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
func (m *Module) registerServerRoutes(router fiber.Router, authMiddleware fiber.Handler, handler *handlers.Handler) {
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

		// Routes that require a provisioned (running) server
		provisioned := servers.Group("", middleware.RequireProvisionedServer())
		{
			provisioned.Put("/:id", handler.Update)
			provisioned.Patch("/:id", handler.Update)

			// Actions
			provisioned.Post("/:id/reboot", handler.Reboot)
			provisioned.Post("/:id/connect", handler.Connect)
			provisioned.Post("/:id/vulnerability-audit", handler.RunVulnerabilityAudit)

			// Services
			provisioned.Get("/:id/services", handler.ListServices)
			provisioned.Get("/:id/services/create", handler.GetAvailableServices)
			provisioned.Post("/:id/services", handler.InstallService)
			provisioned.Post("/:id/services/:serviceId", handler.ServiceOperation)

			// PHP
			provisioned.Get("/:id/php", handler.ListPhpVersions)
			provisioned.Get("/:id/php-versions", handler.ListInstalledPhpVersions)
			provisioned.Get("/:id/php/opcache/defaults", handler.GetOpcacheDefaults)
			provisioned.Get("/:id/php/:phpId/opcache/status", handler.GetOpcacheStatus)
			provisioned.Post("/:id/php/:phpId/opcache/reset", handler.ResetOpcache)
			provisioned.Post("/:id/php/:phpId/opcache/configure", handler.ConfigureOpcache)

			// Composer Packages
			provisioned.Get("/:id/packages", handler.GetComposerAuth)
			provisioned.Put("/:id/packages", handler.UpdateComposerAuth)

			// Firewall Rules
			provisioned.Get("/:id/firewall-rules", handler.ListFirewallRules)
			provisioned.Post("/:id/firewall-rules", handler.CreateFirewallRule)
			provisioned.Put("/:id/firewall-rules/:ruleId", handler.UpdateFirewallRule)
			provisioned.Delete("/:id/firewall-rules/:ruleId", handler.DeleteFirewallRule)

			// Cron Jobs
			provisioned.Get("/:id/crons", handler.ListCrons)
			provisioned.Post("/:id/crons", handler.CreateCron)
			provisioned.Put("/:id/crons/:cronId", handler.UpdateCron)
			provisioned.Delete("/:id/crons/:cronId", handler.DeleteCron)

			// Daemons
			provisioned.Get("/:id/daemons", handler.ListDaemons)
			provisioned.Post("/:id/daemons", handler.CreateDaemon)
			provisioned.Post("/:id/daemons/sync", handler.SyncDaemons)
			provisioned.Put("/:id/daemons/:daemonId", handler.UpdateDaemon)
			provisioned.Post("/:id/daemons/:daemonId/restart", handler.RestartDaemon)
			provisioned.Delete("/:id/daemons/:daemonId", handler.DeleteDaemon)

			// SSH Keys (server-specific)
			provisioned.Get("/:id/ssh-keys", handler.ListServerSSHKeys)
			provisioned.Post("/:id/ssh-keys", handler.AttachSSHKey)
			provisioned.Delete("/:id/ssh-keys/:sshKeyId", handler.DetachSSHKey)

			// Metrics
			provisioned.Get("/:id/metrics", handler.GetMetrics)
			provisioned.Get("/:id/metrics/latest", handler.GetLatestMetric)

			// Logs
			provisioned.Get("/:id/logs", handler.ListLogs)
			provisioned.Get("/:id/logs/:log", handler.GetLogContent)
		}
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
