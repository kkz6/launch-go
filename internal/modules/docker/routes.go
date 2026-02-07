package docker

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/middleware"
	"github.com/kkz6/launch-go/internal/modules/docker/handlers"
)

// RegisterRoutes registers all docker module routes
func (m *Module) RegisterRoutes(router fiber.Router, authMiddleware fiber.Handler) {
	handler := handlers.NewDockerServiceHandler(m.service)
	registryHandler := handlers.NewDockerRegistryHandler(m.repos.DockerRegistry())
	envHandler := handlers.NewDockerEnvHandler(m.repos.EnvVar(), m.repos.DockerService())
	volumeHandler := handlers.NewDockerVolumeHandler(m.repos.Volume(), m.repos.DockerService())
	portHandler := handlers.NewDockerPortHandler(m.repos.Port(), m.repos.DockerService())
	domainHandler := handlers.NewDockerDomainHandler(m.repos.Domain(), m.repos.DockerService())
	composeHandler := handlers.NewDockerComposeHandler(m.service)

	// Docker Registries (team-scoped)
	registries := router.Group("/docker-registries",
		authMiddleware, middleware.TeamScope(), middleware.VerifySubscription())
	{
		registries.Get("/", registryHandler.List)
		registries.Post("/", registryHandler.Create)
		registries.Get("/:id", registryHandler.Show)
		registries.Put("/:id", registryHandler.Update)
		registries.Delete("/:id", registryHandler.Delete)
	}

	// Docker Services
	dockerServices := router.Group("/servers/:serverId/docker-services",
		authMiddleware, middleware.TeamScope(), middleware.VerifySubscription())
	{
		dockerServices.Get("/", handler.List)
		dockerServices.Post("/", handler.Create)
		dockerServices.Get("/:id", handler.Show)
		dockerServices.Put("/:id", handler.Update)
		dockerServices.Delete("/:id", handler.Delete)
		dockerServices.Post("/:id/deploy", handler.Deploy)
		dockerServices.Post("/:id/stop", handler.Stop)
		dockerServices.Post("/:id/start", handler.Start)
		dockerServices.Post("/:id/restart", handler.Restart)

		// Domains
		dockerServices.Get("/:id/domains", domainHandler.List)
		dockerServices.Post("/:id/domains", domainHandler.Create)
		dockerServices.Put("/:id/domains/:domainId", domainHandler.Update)
		dockerServices.Delete("/:id/domains/:domainId", domainHandler.Delete)

		// Env Vars
		dockerServices.Get("/:id/env", envHandler.List)
		dockerServices.Put("/:id/env", envHandler.BulkReplace)

		// Volumes
		dockerServices.Get("/:id/volumes", volumeHandler.List)
		dockerServices.Post("/:id/volumes", volumeHandler.Create)
		dockerServices.Delete("/:id/volumes/:volumeId", volumeHandler.Delete)

		// Ports
		dockerServices.Get("/:id/ports", portHandler.List)
		dockerServices.Post("/:id/ports", portHandler.Create)
		dockerServices.Delete("/:id/ports/:portId", portHandler.Delete)
	}

	// Docker Compose
	compose := router.Group("/servers/:serverId/docker-compose",
		authMiddleware, middleware.TeamScope(), middleware.VerifySubscription())
	{
		compose.Post("/preview", composeHandler.Preview)
		compose.Post("/import", composeHandler.Import)
	}
}
