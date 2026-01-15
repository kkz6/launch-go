package server

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/middleware"
)

// RegisterRoutes registers all server module routes
func (m *Module) RegisterRoutes(router fiber.Router, authMiddleware fiber.Handler) {
	m.registerServerProviderRoutes(router, authMiddleware)
	m.registerServerRoutes(router, authMiddleware)
	m.registerSshKeyRoutes(router, authMiddleware)
}

// registerServerProviderRoutes registers server provider routes
func (m *Module) registerServerProviderRoutes(router fiber.Router, authMiddleware fiber.Handler) {
	providers := router.Group("/server-providers", authMiddleware, middleware.TeamScope())
	{
		providers.Get("/", m.handler.ListServerProviders)
	}
}

// registerServerRoutes registers all server-related routes
func (m *Module) registerServerRoutes(router fiber.Router, authMiddleware fiber.Handler) {
	servers := router.Group("/servers", authMiddleware, middleware.TeamScope())
	{
		// Options (must be before /:id routes)
		servers.Get("/create-options", m.handler.GetCreateOptions)

		// CRUD
		servers.Get("/", m.handler.List)
		servers.Post("/", m.handler.Create)
		servers.Get("/:id", m.handler.Show)
		servers.Get("/:id/page", m.handler.ShowPage)
		servers.Put("/:id", m.handler.Update)
		servers.Delete("/:id", m.handler.Delete)

		// Actions
		servers.Post("/:id/reboot", m.handler.Reboot)
		servers.Post("/:id/connect", m.handler.Connect)
		servers.Post("/:id/archive", m.handler.Archive)
		servers.Post("/:id/unarchive", m.handler.Unarchive)
		servers.Post("/:id/vulnerability-audit", m.handler.RunVulnerabilityAudit)

		// Services
		servers.Get("/:id/services", m.handler.ListServices)
		servers.Get("/:id/services/create", m.handler.GetAvailableServices)
		servers.Post("/:id/services", m.handler.InstallService)
		servers.Get("/:id/services/:serviceId/status", m.handler.GetServiceStatus)
		servers.Post("/:id/services/:serviceId/status", m.handler.CheckServiceStatus)
		servers.Post("/:id/services/:serviceId", m.handler.ServiceOperation)

		// PHP
		servers.Get("/:id/php", m.handler.ListPhpVersions)
		servers.Get("/:id/php/opcache/defaults", m.handler.GetOpcacheDefaults)
		servers.Get("/:id/php/:phpId/opcache/status", m.handler.GetOpcacheStatus)
		servers.Post("/:id/php/:phpId/opcache/reset", m.handler.ResetOpcache)
		servers.Post("/:id/php/:phpId/opcache/configure", m.handler.ConfigureOpcache)

		// Composer Packages
		servers.Get("/:id/packages", m.handler.GetComposerAuth)
		servers.Put("/:id/packages", m.handler.UpdateComposerAuth)

		// Firewall Rules
		servers.Get("/:id/firewall-rules", m.handler.ListFirewallRules)
		servers.Post("/:id/firewall-rules", m.handler.CreateFirewallRule)
		servers.Put("/:id/firewall-rules/:ruleId", m.handler.UpdateFirewallRule)
		servers.Delete("/:id/firewall-rules/:ruleId", m.handler.DeleteFirewallRule)

		// Cron Jobs
		servers.Get("/:id/crons", m.handler.ListCrons)
		servers.Post("/:id/crons", m.handler.CreateCron)
		servers.Put("/:id/crons/:cronId", m.handler.UpdateCron)
		servers.Delete("/:id/crons/:cronId", m.handler.DeleteCron)

		// Daemons
		servers.Get("/:id/daemons", m.handler.ListDaemons)
		servers.Post("/:id/daemons", m.handler.CreateDaemon)
		servers.Post("/:id/daemons/sync", m.handler.SyncDaemons)
		servers.Put("/:id/daemons/:daemonId", m.handler.UpdateDaemon)
		servers.Delete("/:id/daemons/:daemonId", m.handler.DeleteDaemon)

		// SSH Keys (server-specific)
		servers.Get("/:id/ssh-keys", m.handler.ListServerSshKeys)
		servers.Post("/:id/ssh-keys", m.handler.AttachSshKey)
		servers.Delete("/:id/ssh-keys/:sshKeyId", m.handler.DetachSshKey)

		// Tasks
		servers.Get("/:id/tasks", m.handler.ListTasks)
		servers.Get("/:id/tasks/latest", m.handler.GetLatestTask)

		// Metrics
		servers.Get("/:id/metrics", m.handler.GetMetrics)
		servers.Get("/:id/metrics/latest", m.handler.GetLatestMetric)

		// Logs
		servers.Get("/:id/logs", m.handler.ListLogs)
		servers.Get("/:id/logs/:log", m.handler.GetLogContent)
	}
}

// registerSshKeyRoutes registers global SSH key routes
func (m *Module) registerSshKeyRoutes(router fiber.Router, authMiddleware fiber.Handler) {
	sshKeys := router.Group("/ssh-keys", authMiddleware, middleware.TeamScope())
	{
		sshKeys.Get("/", m.handler.ListSshKeys)
		sshKeys.Post("/", m.handler.CreateSshKey)
		sshKeys.Delete("/:sshKeyId", m.handler.DeleteSshKey)
	}
}
