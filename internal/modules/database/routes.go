package database

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/middleware"
	"github.com/kkz6/launch-go/internal/modules/database/handlers"
)

// RegisterRoutes registers all database module routes
func (m *Module) RegisterRoutes(router fiber.Router, authMiddleware fiber.Handler) {
	handler := handlers.NewHandler(m.service)

	m.registerDatabaseRoutes(router, authMiddleware, handler)
	m.registerDatabaseUserRoutes(router, authMiddleware, handler)
}

// registerDatabaseRoutes registers database routes under /servers/:id/databases
func (m *Module) registerDatabaseRoutes(router fiber.Router, authMiddleware fiber.Handler, handler *handlers.Handler) {
	databases := router.Group("/servers/:serverId/databases", authMiddleware, middleware.TeamScope())
	{
		databases.Get("/", handler.ListDatabases)
		databases.Post("/", handler.CreateDatabase)
		databases.Get("/:id", handler.GetDatabase)
		databases.Delete("/:id", handler.DeleteDatabase)
		databases.Post("/sync", handler.SyncDatabases)
	}
}

// registerDatabaseUserRoutes registers database user routes under /servers/:id/database-users
func (m *Module) registerDatabaseUserRoutes(router fiber.Router, authMiddleware fiber.Handler, handler *handlers.Handler) {
	users := router.Group("/servers/:serverId/database-users", authMiddleware, middleware.TeamScope())
	{
		users.Get("/", handler.ListDatabaseUsers)
		users.Post("/", handler.CreateDatabaseUser)
		users.Get("/:id", handler.GetDatabaseUser)
		users.Put("/:id", handler.UpdateDatabaseUser)
		users.Delete("/:id", handler.DeleteDatabaseUser)
	}
}
