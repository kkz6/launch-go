package managedservice

import (
	gofiber "github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/middleware"
	"github.com/kkz6/launch-go/internal/modules/managedservice/dto"
	"github.com/kkz6/launch-go/internal/modules/managedservice/handlers"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// RegisterRoutes wires the managed-service HTTP routes.
func (m *Module) RegisterRoutes(router gofiber.Router, authMiddleware gofiber.Handler) {
	auth := middleware.Append(
		middleware.AuthenticatedChain(authMiddleware),
		middleware.RequireProvisionedServer("serverId"),
	)

	h := handlers.NewManagedServiceHandler(m.service)

	g := router.Group("/servers/:serverId/managed-services", auth...)
	g.Get("/", fiberutil.IndexNested("serverId", "Managed services retrieved", m.service.ListByServer))
	g.Post("/", fiberutil.CreateNested[dto.InstallManagedServiceRequest]("serverId", "Managed service will be installed shortly", m.service.Install))

	g.Get("/:kind", h.Show)
	g.Delete("/:kind", h.Uninstall)
	g.Post("/:kind/start", h.Start)
	g.Post("/:kind/stop", h.Stop)
	g.Post("/:kind/restart", h.Restart)
	g.Get("/:kind/logs", h.Logs)
}
