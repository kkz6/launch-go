package database

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/middleware"
)

// RegisterRoutes registers all database module routes
func (m *Module) RegisterRoutes(router fiber.Router, authMiddleware fiber.Handler) {
	m.registerDatabaseRoutes(router, authMiddleware)
	m.registerDatabaseUserRoutes(router, authMiddleware)
}

// registerDatabaseRoutes registers database routes under /servers/:id/databases
func (m *Module) registerDatabaseRoutes(router fiber.Router, authMiddleware fiber.Handler) {
	databases := router.Group("/servers/:serverId/databases", authMiddleware, middleware.TeamScope())
	{
		databases.Get("/", m.handler.ListDatabases)
		databases.Post("/", m.handler.CreateDatabase)
		databases.Get("/:id", m.handler.GetDatabase)
		databases.Delete("/:id", m.handler.DeleteDatabase)
		databases.Post("/sync", m.handler.SyncDatabases)
	}
}

// registerDatabaseUserRoutes registers database user routes under /servers/:id/database-users
func (m *Module) registerDatabaseUserRoutes(router fiber.Router, authMiddleware fiber.Handler) {
	users := router.Group("/servers/:serverId/database-users", authMiddleware, middleware.TeamScope())
	{
		users.Get("/", m.handler.ListDatabaseUsers)
		users.Post("/", m.handler.CreateDatabaseUser)
		users.Get("/:id", m.handler.GetDatabaseUser)
		users.Put("/:id", m.handler.UpdateDatabaseUser)
		users.Delete("/:id", m.handler.DeleteDatabaseUser)
	}
}
