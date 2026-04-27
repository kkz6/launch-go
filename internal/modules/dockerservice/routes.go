package dockerservice

import (
	gofiber "github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/middleware"
	"github.com/kkz6/launch-go/internal/modules/dockerservice/dto"
	"github.com/kkz6/launch-go/internal/modules/dockerservice/handlers"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// RegisterRoutes wires the docker-service HTTP routes.
func (m *Module) RegisterRoutes(router gofiber.Router, authMiddleware gofiber.Handler) {
	auth := middleware.Append(
		middleware.AuthenticatedChain(authMiddleware),
		middleware.RequireProvisionedServer("serverId"),
	)

	h := handlers.NewDockerServiceHandler(m.service)

	g := router.Group("/servers/:serverId/docker-services", auth...)
	g.Get("/", fiberutil.IndexNested("serverId", "Docker services retrieved", m.service.ListByServer))
	g.Post("/", fiberutil.CreateNested[dto.InstallDockerServiceRequest]("serverId", "Docker service will be installed shortly", m.service.Install))

	g.Get("/:kind", h.Show)
	g.Delete("/:kind", h.Uninstall)
	g.Post("/:kind/start", h.Start)
	g.Post("/:kind/stop", h.Stop)
	g.Post("/:kind/restart", h.Restart)
	g.Get("/:kind/logs", h.Logs)
}
