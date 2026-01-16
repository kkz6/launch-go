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
		DB:         deps.DB,
		Queue:      deps.Queue,
		Dispatcher: deps.Dispatcher,
		Logger:     deps.Logger,
	}
	handler := handlers.NewHandler(m.service, taskRunnerDeps, siteCounter)

	m.registerServerProviderRoutes(router, authMiddleware, handler)
	m.registerServerRoutes(router, authMiddleware, handler)
	m.registerSshKeyRoutes(router, authMiddleware, handler)
}

// registerServerProviderRoutes registers server provider routes
func (m *Module) registerServerProviderRoutes(router fiber.Router, authMiddleware fiber.Handler, handler *handlers.Handler) {
	providers := router.Group("/server-providers", authMiddleware, middleware.TeamScope())
	{
		providers.Get("/", handler.ListServerProviders)
	}
}

// registerServerRoutes registers all server-related routes
func (m *Module) registerServerRoutes(router fiber.Router, authMiddleware fiber.Handler, handler *handlers.Handler) {
	servers := router.Group("/servers", authMiddleware, middleware.TeamScope())
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
		servers.Put("/:id", handler.Update)
		servers.Patch("/:id", handler.Update)
		servers.Delete("/:id", handler.Delete)

		// Actions
		servers.Post("/:id/reboot", handler.Reboot)
		servers.Post("/:id/connect", handler.Connect)
		servers.Post("/:id/archive", handler.Archive)
		servers.Post("/:id/unarchive", handler.Unarchive)
		servers.Post("/:id/vulnerability-audit", handler.RunVulnerabilityAudit)

		// Services
		servers.Get("/:id/services", handler.ListServices)
		servers.Get("/:id/services/create", handler.GetAvailableServices)
		servers.Post("/:id/services", handler.InstallService)
		servers.Post("/:id/services/:serviceId", handler.ServiceOperation)

		// PHP
		servers.Get("/:id/php", handler.ListPhpVersions)
		servers.Get("/:id/php/opcache/defaults", handler.GetOpcacheDefaults)
		servers.Get("/:id/php/:phpId/opcache/status", handler.GetOpcacheStatus)
		servers.Post("/:id/php/:phpId/opcache/reset", handler.ResetOpcache)
		servers.Post("/:id/php/:phpId/opcache/configure", handler.ConfigureOpcache)

		// Composer Packages
		servers.Get("/:id/packages", handler.GetComposerAuth)
		servers.Put("/:id/packages", handler.UpdateComposerAuth)

		// Firewall Rules
		servers.Get("/:id/firewall-rules", handler.ListFirewallRules)
		servers.Post("/:id/firewall-rules", handler.CreateFirewallRule)
		servers.Put("/:id/firewall-rules/:ruleId", handler.UpdateFirewallRule)
		servers.Delete("/:id/firewall-rules/:ruleId", handler.DeleteFirewallRule)

		// Cron Jobs
		servers.Get("/:id/crons", handler.ListCrons)
		servers.Post("/:id/crons", handler.CreateCron)
		servers.Put("/:id/crons/:cronId", handler.UpdateCron)
		servers.Delete("/:id/crons/:cronId", handler.DeleteCron)

		// Daemons
		servers.Get("/:id/daemons", handler.ListDaemons)
		servers.Post("/:id/daemons", handler.CreateDaemon)
		servers.Post("/:id/daemons/sync", handler.SyncDaemons)
		servers.Put("/:id/daemons/:daemonId", handler.UpdateDaemon)
		servers.Delete("/:id/daemons/:daemonId", handler.DeleteDaemon)

		// SSH Keys (server-specific)
		servers.Get("/:id/ssh-keys", handler.ListServerSshKeys)
		servers.Post("/:id/ssh-keys", handler.AttachSshKey)
		servers.Delete("/:id/ssh-keys/:sshKeyId", handler.DetachSshKey)

		// Tasks
		servers.Get("/:id/tasks", handler.ListTasks)
		servers.Get("/:id/tasks/latest", handler.GetLatestTask)

		// Metrics
		servers.Get("/:id/metrics", handler.GetMetrics)
		servers.Get("/:id/metrics/latest", handler.GetLatestMetric)

		// Logs
		servers.Get("/:id/logs", handler.ListLogs)
		servers.Get("/:id/logs/:log", handler.GetLogContent)
	}
}

// registerSshKeyRoutes registers global SSH key routes
func (m *Module) registerSshKeyRoutes(router fiber.Router, authMiddleware fiber.Handler, handler *handlers.Handler) {
	sshKeys := router.Group("/ssh-keys", authMiddleware, middleware.TeamScope())
	{
		sshKeys.Get("/", handler.ListSshKeys)
		sshKeys.Post("/", handler.CreateSshKey)
		sshKeys.Delete("/:sshKeyId", handler.DeleteSshKey)
	}
}

// RegisterWebhookRoutes registers all webhook routes (no auth required, uses signed URLs)
func (m *Module) RegisterWebhookRoutes(router fiber.Router) {
	deps := m.Deps()
	webhookHandler := handlers.NewTaskWebhookHandler(m.repo, deps.Config.App.Key, deps.Queue, deps.Logger)
	provisionScriptHandler := handlers.NewProvisionScriptHandler(m.repo, m.service)

	m.registerTaskWebhookRoutes(router, webhookHandler)
	m.registerProvisionScriptRoutes(router, provisionScriptHandler)
}

// registerTaskWebhookRoutes registers task completion webhook routes
func (m *Module) registerTaskWebhookRoutes(router fiber.Router, handler *handlers.TaskWebhookHandler) {
	webhooks := router.Group("/webhooks/tasks")
	webhooks.Post("/:id/finished", handler.MarkAsFinished)
	webhooks.Post("/:id/failed", handler.MarkAsFailed)
	webhooks.Post("/:id/timeout", handler.MarkAsTimeout)
}

// registerProvisionScriptRoutes registers provision script routes (signed URL protected)
func (m *Module) registerProvisionScriptRoutes(router fiber.Router, handler *handlers.ProvisionScriptHandler) {
	router.Get("/servers/:id/provision-script",
		signedurl.RequireSignedURL(nil),
		handler.GetProvisionScript,
	)
}
