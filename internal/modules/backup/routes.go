package backup

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/middleware"
	"github.com/kkz6/launch-go/internal/modules/backup/handlers"
)

// RegisterRoutes registers the backup module routes
func (m *Module) RegisterRoutes(router fiber.Router, authMiddleware fiber.Handler) {
	// Create services
	svc := m.createServices()

	// Create handlers
	backupHandler := handlers.NewBackupHandler(svc.Backup())
	storageProviderHandler := handlers.NewStorageProviderHandler(svc.StorageProvider())

	m.registerBackupRoutes(router, authMiddleware, backupHandler)
	m.registerStorageProviderRoutes(router, authMiddleware, storageProviderHandler)
}

// registerBackupRoutes registers server backup routes
func (m *Module) registerBackupRoutes(router fiber.Router, authMiddleware fiber.Handler, handler *handlers.BackupHandler) {
	serverBackups := router.Group("/servers/:serverId/backups", authMiddleware, middleware.TeamScope())
	{
		serverBackups.Get("/", handler.ListBackups)
		serverBackups.Post("/", handler.CreateBackup)
		serverBackups.Get("/:id", handler.ShowBackup)
		serverBackups.Put("/:id", handler.UpdateBackup)
		serverBackups.Delete("/:id", handler.DeleteBackup)
		serverBackups.Post("/:id/run", handler.RunManualBackup)
	}
}

// registerStorageProviderRoutes registers storage provider routes
func (m *Module) registerStorageProviderRoutes(router fiber.Router, authMiddleware fiber.Handler, handler *handlers.StorageProviderHandler) {
	storageProviders := router.Group("/storage-providers", authMiddleware, middleware.TeamScope())
	{
		storageProviders.Get("/", handler.ListStorageProviders)
		storageProviders.Get("/dropdown", handler.ListStorageProvidersForDropdown)
		storageProviders.Get("/:id", handler.ShowStorageProvider)
		storageProviders.Post("/:provider/connect", handler.ConnectStorageProvider)
		storageProviders.Put("/:provider", handler.UpdateStorageProvider)
		storageProviders.Delete("/:provider", handler.DeleteStorageProvider)
	}
}

// RegisterWebhookRoutes registers webhook routes that don't require authentication
func (m *Module) RegisterWebhookRoutes(router fiber.Router) {
	// Create services
	svc := m.createServices()

	// Create handler
	backupJobHandler := handlers.NewBackupJobHandler(svc.BackupJob())

	m.registerWebhookRoutes(router, backupJobHandler)
}

// registerWebhookRoutes registers backup job webhook routes
func (m *Module) registerWebhookRoutes(router fiber.Router, handler *handlers.BackupJobHandler) {
	router.Post("/backup/:backup/:token", handler.CreateBackupJob)
}
