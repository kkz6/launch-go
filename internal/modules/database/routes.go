package database

import (
	gofiber "github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/middleware"
	"github.com/kkz6/launch-go/internal/modules/database/dto"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// RegisterRoutes registers all database module routes. All endpoints are
// server-scoped (parent param "serverId") and use the framework helpers
// from pkg/fiber, so the wiring here is effectively a route table.
func (m *Module) RegisterRoutes(router gofiber.Router, authMiddleware gofiber.Handler) {
	svc := m.service

	auth := middleware.Append(middleware.AuthenticatedChain(authMiddleware), middleware.RequireProvisionedServer("serverId"))

	databases := router.Group("/servers/:serverId/databases", auth...)
	databases.Get("/", fiberutil.IndexNested("serverId", "Databases retrieved", svc.ListDatabases))
	databases.Post("/", fiberutil.CreateNested[dto.CreateDatabaseRequest]("serverId", "Database will be created shortly", svc.CreateDatabase))
	databases.Get("/:id", fiberutil.ShowNested("serverId", "id", "Database retrieved", svc.GetDatabase))
	databases.Delete("/:id", fiberutil.DeleteNested("serverId", "id", svc.DeleteDatabase))
	databases.Post("/sync", fiberutil.ActionNested("serverId", "Database sync started", svc.SyncDatabases))

	users := router.Group("/servers/:serverId/database-users", auth...)
	users.Get("/", fiberutil.IndexNested("serverId", "Database users retrieved", svc.ListDatabaseUsers))
	users.Post("/", fiberutil.CreateNested[dto.CreateDatabaseUserRequest]("serverId", "Database user will be created shortly", svc.CreateDatabaseUser))
	users.Get("/:id", fiberutil.ShowNested("serverId", "id", "Database user retrieved", svc.GetDatabaseUser))
	users.Put("/:id", fiberutil.UpdateNested[dto.UpdateDatabaseUserRequest]("serverId", "id", "Database user will be updated shortly", svc.UpdateDatabaseUser))
	users.Delete("/:id", fiberutil.DeleteNested("serverId", "id", svc.DeleteDatabaseUser))
}
