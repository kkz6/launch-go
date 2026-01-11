package database

import (
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/queue"
	"github.com/kkz6/launch-go/internal/websocket"
)

// Module represents the database module
type Module struct {
	handler    *Handler
	service    *Service
	repository *Repository
}

// NewModule creates a new database module
func NewModule(db *gorm.DB, serverRepo ServerRepository, queueClient *queue.Client, ws *websocket.Hub, logger *zerolog.Logger) *Module {
	repo := NewRepository(db)
	service := NewService(repo, serverRepo, queueClient, ws, logger)
	handler := NewHandler(service)

	return &Module{
		handler:    handler,
		service:    service,
		repository: repo,
	}
}

// Service returns the database service
func (m *Module) Service() *Service {
	return m.service
}

// Repository returns the database repository
func (m *Module) Repository() *Repository {
	return m.repository
}

// RegisterRoutes registers the database routes
func (m *Module) RegisterRoutes(router fiber.Router, authMiddleware fiber.Handler) {
	// Database routes under /servers/:serverId/databases
	databases := router.Group("/servers/:serverId/databases", authMiddleware)

	databases.Get("/", m.handler.ListDatabases)
	databases.Post("/", m.handler.CreateDatabase)
	databases.Get("/:id", m.handler.GetDatabase)
	databases.Delete("/:id", m.handler.DeleteDatabase)
	databases.Post("/sync", m.handler.SyncDatabases)

	// Database user routes under /servers/:serverId/database-users
	users := router.Group("/servers/:serverId/database-users", authMiddleware)

	users.Get("/", m.handler.ListDatabaseUsers)
	users.Post("/", m.handler.CreateDatabaseUser)
	users.Get("/:id", m.handler.GetDatabaseUser)
	users.Put("/:id", m.handler.UpdateDatabaseUser)
	users.Delete("/:id", m.handler.DeleteDatabaseUser)
}

// AutoMigrate runs auto-migration for database models
func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(&Database{}, &DatabaseUser{}, &DatabaseDatabaseUser{})
}
