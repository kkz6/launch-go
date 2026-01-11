package backup

import (
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/middleware"
	"github.com/kkz6/launch-go/internal/queue"
	"github.com/kkz6/launch-go/internal/websocket"
)

// Module represents the backup module
type Module struct {
	handler *Handler
	service *Service
	repo    *Repository
}

// NewModule creates a new backup module
func NewModule(db *gorm.DB, queueClient *queue.Client, ws *websocket.Hub, logger *zerolog.Logger) *Module {
	repo := NewRepository(db)
	service := NewService(repo, queueClient, ws, logger)
	handler := NewHandler(service)

	return &Module{
		handler: handler,
		service: service,
		repo:    repo,
	}
}

// RegisterRoutes registers the backup module routes
func (m *Module) RegisterRoutes(router fiber.Router, authMiddleware fiber.Handler) {
	// Server backup routes (require authentication and team scope)
	serverBackups := router.Group("/servers/:serverId/backups", authMiddleware, middleware.TeamScope())
	{
		serverBackups.Get("/", m.handler.ListBackups)
		serverBackups.Post("/", m.handler.CreateBackup)
		serverBackups.Get("/:id", m.handler.ShowBackup)
		serverBackups.Put("/:id", m.handler.UpdateBackup)
		serverBackups.Delete("/:id", m.handler.DeleteBackup)
		serverBackups.Post("/:id/run", m.handler.RunManualBackup)
	}

	// Storage provider routes (require authentication and team scope)
	storageProviders := router.Group("/storage-providers", authMiddleware, middleware.TeamScope())
	{
		storageProviders.Get("/", m.handler.ListStorageProviders)
		storageProviders.Get("/dropdown", m.handler.ListStorageProvidersForDropdown)
		storageProviders.Get("/:id", m.handler.ShowStorageProvider)
		storageProviders.Post("/:provider/connect", m.handler.ConnectStorageProvider)
		storageProviders.Put("/:provider", m.handler.UpdateStorageProvider)
		storageProviders.Delete("/:provider", m.handler.DeleteStorageProvider)
	}
}

// RegisterWebhookRoutes registers webhook routes that don't require authentication
func (m *Module) RegisterWebhookRoutes(router fiber.Router) {
	// Backup job webhook (called by agent)
	router.Post("/backup/:backup/:token", m.handler.CreateBackupJob)
}

// GetService returns the backup service
func (m *Module) GetService() *Service {
	return m.service
}

// GetRepository returns the backup repository
func (m *Module) GetRepository() *Repository {
	return m.repo
}

// GetHandler returns the backup handler
func (m *Module) GetHandler() *Handler {
	return m.handler
}

// AutoMigrate runs database migrations for the backup module
func (m *Module) AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&Backup{},
		&BackupJob{},
		&StorageProvider{},
		&BackupDatabase{},
	)
}
