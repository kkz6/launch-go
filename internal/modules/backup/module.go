package backup

import (
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/middleware"
	"github.com/kkz6/launch-go/internal/modules/backup/handlers"
	"github.com/kkz6/launch-go/internal/modules/backup/models"
	"github.com/kkz6/launch-go/internal/modules/backup/repositories"
	"github.com/kkz6/launch-go/internal/modules/backup/services"
	"github.com/kkz6/launch-go/internal/queue"
	"github.com/kkz6/launch-go/internal/websocket"
)

// Module represents the backup module
type Module struct {
	// Handlers
	backupHandler          *handlers.BackupHandler
	backupJobHandler       *handlers.BackupJobHandler
	storageProviderHandler *handlers.StorageProviderHandler

	// Services
	backupService          *services.BackupService
	backupJobService       *services.BackupJobService
	storageProviderService *services.StorageProviderService
	agentConfigService     *services.AgentConfigService

	// Repositories
	backupRepo          *repositories.BackupRepository
	backupJobRepo       *repositories.BackupJobRepository
	storageProviderRepo *repositories.StorageProviderRepository
}

// NewModule creates a new backup module
func NewModule(db *gorm.DB, queueClient *queue.Client, ws *websocket.Hub, logger *zerolog.Logger) *Module {
	// Initialize repositories
	backupRepo := repositories.NewBackupRepository(db)
	backupJobRepo := repositories.NewBackupJobRepository(db)
	storageProviderRepo := repositories.NewStorageProviderRepository(db)

	// Initialize services
	backupService := services.NewBackupService(backupRepo, queueClient, ws, logger)
	backupJobService := services.NewBackupJobService(backupJobRepo, backupRepo, ws, logger)
	storageProviderService := services.NewStorageProviderService(storageProviderRepo, queueClient, logger)
	agentConfigService := services.NewAgentConfigService(backupRepo, storageProviderService, logger)

	// Initialize handlers
	backupHandler := handlers.NewBackupHandler(backupService)
	backupJobHandler := handlers.NewBackupJobHandler(backupJobService)
	storageProviderHandler := handlers.NewStorageProviderHandler(storageProviderService)

	return &Module{
		// Handlers
		backupHandler:          backupHandler,
		backupJobHandler:       backupJobHandler,
		storageProviderHandler: storageProviderHandler,

		// Services
		backupService:          backupService,
		backupJobService:       backupJobService,
		storageProviderService: storageProviderService,
		agentConfigService:     agentConfigService,

		// Repositories
		backupRepo:          backupRepo,
		backupJobRepo:       backupJobRepo,
		storageProviderRepo: storageProviderRepo,
	}
}

// RegisterRoutes registers the backup module routes
func (m *Module) RegisterRoutes(router fiber.Router, authMiddleware fiber.Handler) {
	// Server backup routes (require authentication and team scope)
	serverBackups := router.Group("/servers/:serverId/backups", authMiddleware, middleware.TeamScope())
	{
		serverBackups.Get("/", m.backupHandler.ListBackups)
		serverBackups.Post("/", m.backupHandler.CreateBackup)
		serverBackups.Get("/:id", m.backupHandler.ShowBackup)
		serverBackups.Put("/:id", m.backupHandler.UpdateBackup)
		serverBackups.Delete("/:id", m.backupHandler.DeleteBackup)
		serverBackups.Post("/:id/run", m.backupHandler.RunManualBackup)
	}

	// Storage provider routes (require authentication and team scope)
	storageProviders := router.Group("/storage-providers", authMiddleware, middleware.TeamScope())
	{
		storageProviders.Get("/", m.storageProviderHandler.ListStorageProviders)
		storageProviders.Get("/dropdown", m.storageProviderHandler.ListStorageProvidersForDropdown)
		storageProviders.Get("/:id", m.storageProviderHandler.ShowStorageProvider)
		storageProviders.Post("/:provider/connect", m.storageProviderHandler.ConnectStorageProvider)
		storageProviders.Put("/:provider", m.storageProviderHandler.UpdateStorageProvider)
		storageProviders.Delete("/:provider", m.storageProviderHandler.DeleteStorageProvider)
	}
}

// RegisterWebhookRoutes registers webhook routes that don't require authentication
func (m *Module) RegisterWebhookRoutes(router fiber.Router) {
	// Backup job webhook (called by agent)
	router.Post("/backup/:backup/:token", m.backupJobHandler.CreateBackupJob)
}

// GetBackupService returns the backup service
func (m *Module) GetBackupService() *services.BackupService {
	return m.backupService
}

// GetBackupJobService returns the backup job service
func (m *Module) GetBackupJobService() *services.BackupJobService {
	return m.backupJobService
}

// GetStorageProviderService returns the storage provider service
func (m *Module) GetStorageProviderService() *services.StorageProviderService {
	return m.storageProviderService
}

// GetAgentConfigService returns the agent config service
func (m *Module) GetAgentConfigService() *services.AgentConfigService {
	return m.agentConfigService
}

// GetBackupRepository returns the backup repository
func (m *Module) GetBackupRepository() *repositories.BackupRepository {
	return m.backupRepo
}

// GetBackupJobRepository returns the backup job repository
func (m *Module) GetBackupJobRepository() *repositories.BackupJobRepository {
	return m.backupJobRepo
}

// GetStorageProviderRepository returns the storage provider repository
func (m *Module) GetStorageProviderRepository() *repositories.StorageProviderRepository {
	return m.storageProviderRepo
}

// AutoMigrate runs database migrations for the backup module
func (m *Module) AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&models.Backup{},
		&models.BackupJob{},
		&models.StorageProvider{},
		&models.BackupDatabase{},
	)
}
