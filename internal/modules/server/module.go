package server

import (
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/middleware"
	"github.com/kkz6/launch-go/internal/modules/server/handlers"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/repositories"
	"github.com/kkz6/launch-go/internal/modules/server/services"
	"github.com/kkz6/launch-go/internal/queue"
	"github.com/kkz6/launch-go/internal/taskrunner"
	"github.com/kkz6/launch-go/internal/websocket"
)

type Module struct {
	handler *handlers.Handler
}

func NewModule(db *gorm.DB, queueClient *queue.Client, ws *websocket.Hub, dispatcher *taskrunner.Dispatcher, logger *zerolog.Logger) *Module {
	repo := repositories.NewRepository(db)
	service := services.NewService(repo, queueClient, ws, dispatcher, logger)
	handler := handlers.NewHandler(service)

	return &Module{
		handler: handler,
	}
}

// AutoMigrate runs database migrations for server models
func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&models.Server{},
		&models.InstalledService{},
		&models.FirewallRule{},
		&models.Cron{},
		&models.Daemon{},
		&models.SshKey{},
		&models.ServerSshKey{},
		&models.Task{},
		&models.Metric{},
	)
}

func (m *Module) RegisterRoutes(router fiber.Router, authMiddleware fiber.Handler) {
	servers := router.Group("/servers", authMiddleware, middleware.TeamScope())

	// Server CRUD
	servers.Get("/", m.handler.List)
	servers.Post("/", m.handler.Create)
	servers.Get("/:id", m.handler.Show)
	servers.Get("/:id/page", m.handler.ShowPage)
	servers.Put("/:id", m.handler.Update)
	servers.Delete("/:id", m.handler.Delete)

	// Server actions
	servers.Post("/:id/reboot", m.handler.Reboot)
	servers.Post("/:id/connect", m.handler.Connect)
	servers.Post("/:id/archive", m.handler.Archive)
	servers.Post("/:id/unarchive", m.handler.Unarchive)

	// Services
	servers.Get("/:id/services", m.handler.ListServices)
	servers.Post("/:id/services", m.handler.InstallService)
	servers.Post("/:id/services/:serviceId", m.handler.ServiceOperation)

	// Firewall rules
	servers.Get("/:id/firewall-rules", m.handler.ListFirewallRules)
	servers.Post("/:id/firewall-rules", m.handler.CreateFirewallRule)
	servers.Put("/:id/firewall-rules/:ruleId", m.handler.UpdateFirewallRule)
	servers.Delete("/:id/firewall-rules/:ruleId", m.handler.DeleteFirewallRule)

	// Cron jobs
	servers.Get("/:id/crons", m.handler.ListCrons)
	servers.Post("/:id/crons", m.handler.CreateCron)
	servers.Put("/:id/crons/:cronId", m.handler.UpdateCron)
	servers.Delete("/:id/crons/:cronId", m.handler.DeleteCron)

	// Daemons
	servers.Get("/:id/daemons", m.handler.ListDaemons)
	servers.Post("/:id/daemons", m.handler.CreateDaemon)
	servers.Put("/:id/daemons/:daemonId", m.handler.UpdateDaemon)
	servers.Delete("/:id/daemons/:daemonId", m.handler.DeleteDaemon)

	// SSH Keys (server-specific)
	servers.Get("/:id/ssh-keys", m.handler.ListServerSshKeys)
	servers.Post("/:id/ssh-keys", m.handler.AttachSshKey)
	servers.Delete("/:id/ssh-keys/:sshKeyId", m.handler.DetachSshKey)

	// Databases
	servers.Get("/:id/databases", m.handler.ListDatabases)
	servers.Post("/:id/databases", m.handler.CreateDatabase)

	// Tasks
	servers.Get("/:id/tasks", m.handler.ListTasks)
	servers.Get("/:id/tasks/latest", m.handler.GetLatestTask)

	// Metrics
	servers.Get("/:id/metrics", m.handler.GetMetrics)
	servers.Get("/:id/metrics/latest", m.handler.GetLatestMetric)

	// Global SSH Keys (not server-specific)
	sshKeys := router.Group("/ssh-keys", authMiddleware, middleware.TeamScope())
	sshKeys.Get("/", m.handler.ListSshKeys)
	sshKeys.Post("/", m.handler.CreateSshKey)
	sshKeys.Delete("/:sshKeyId", m.handler.DeleteSshKey)
}
