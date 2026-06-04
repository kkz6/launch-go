package backup

import (
	gofiber "github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/middleware"
	"github.com/kkz6/launch-go/internal/modules/backup/dto"
	"github.com/kkz6/launch-go/internal/modules/backup/handlers"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// RegisterRoutes registers the backup module routes. Server-scoped backup
// CRUD goes through the framework helpers; storage-provider routes use a
// dual-purpose `:provider` path parameter so they stay hand-written.
func (m *Module) RegisterRoutes(router gofiber.Router, authMiddleware gofiber.Handler) {
	svc := m.createServices()

	storageProviderHandler := handlers.NewStorageProviderHandler(svc.StorageProvider())

	// Server-scoped backup CRUD.
	auth := middleware.Append(middleware.AuthenticatedChain(authMiddleware), middleware.RequireProvisionedServer("serverId"))
	backups := router.Group("/servers/:serverId/backups", auth...)
	backups.Get("/", fiberutil.IndexNested("serverId", "Backups retrieved", svc.Backup().ListBackups))
	backups.Post("/", middleware.Can("backup.create"), fiberutil.CreateNested[dto.CreateBackupRequest]("serverId", "Backup created successfully", svc.Backup().CreateBackup))
	backups.Get("/:id", fiberutil.ShowNested("serverId", "id", "Backup retrieved", svc.Backup().GetBackup))
	backups.Put("/:id", middleware.Can("backup.update"), fiberutil.UpdateNested[dto.UpdateBackupRequest]("serverId", "id", "Backup updated successfully", svc.Backup().UpdateBackup))
	backups.Delete("/:id", middleware.Can("backup.delete"), fiberutil.DeleteNested("serverId", "id", svc.Backup().DeleteBackup))
	backups.Post("/:id/run", middleware.Can("backup.run"), fiberutil.ActionItemNested("serverId", "id", "Backup queued for execution", svc.Backup().RunBackup))

	// Team-scoped storage providers. List / dropdown / show fit the
	// framework helpers; connect / update / delete have unusual path
	// semantics and live in the bespoke handler.
	storage := router.Group("/storage-providers", middleware.AuthenticatedChain(authMiddleware)...)
	storage.Get("/", fiberutil.Index("Storage providers retrieved", svc.StorageProvider().ListStorageProviders))
	storage.Get("/dropdown", fiberutil.Index("Storage providers retrieved", svc.StorageProvider().ListStorageProvidersDropdown))
	storage.Get("/:id", fiberutil.Show("Storage provider retrieved", svc.StorageProvider().GetStorageProvider))
	storage.Post("/:provider/connect", middleware.Can("storage_provider.create"), fiberutil.Handler(storageProviderHandler.ConnectStorageProvider))
	storage.Put("/:provider", middleware.Can("storage_provider.update"), storageProviderHandler.UpdateStorageProvider)
	storage.Delete("/:provider", middleware.Can("storage_provider.delete"), storageProviderHandler.DeleteStorageProvider)
}

// RegisterWebhookRoutes registers webhook routes that don't require
// authentication.
func (m *Module) RegisterWebhookRoutes(router gofiber.Router) {
	svc := m.createServices()
	jobHandler := handlers.NewBackupJobHandler(svc.BackupJob())
	router.Post("/backup/:backup/:token", fiberutil.Validate(jobHandler.CreateBackupJob))
}
