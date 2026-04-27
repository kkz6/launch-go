package dockerapp

import (
	gofiber "github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/middleware"
	"github.com/kkz6/launch-go/internal/modules/dockerapp/dto"
	"github.com/kkz6/launch-go/internal/modules/dockerapp/handlers"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// RegisterRoutes wires the dockerapp HTTP routes.
func (m *Module) RegisterRoutes(router gofiber.Router, authMiddleware gofiber.Handler) {
	svc := m.service
	h := handlers.NewHandler(svc)

	auth := middleware.Append(
		middleware.AuthenticatedChain(authMiddleware),
		middleware.RequireProvisionedServer("serverId"),
	)

	apps := router.Group("/servers/:serverId/apps", auth...)
	apps.Get("/", fiberutil.IndexNested("serverId", "Applications retrieved", svc.ListByServer))
	apps.Post("/", fiberutil.CreateNested[dto.CreateAppRequest]("serverId", "Application will be deployed shortly", svc.Create))
	apps.Get("/:id", fiberutil.ShowNested("serverId", "id", "Application retrieved", svc.Get))
	apps.Put("/:id", fiberutil.UpdateNested[dto.UpdateAppRequest]("serverId", "id", "Application updated", svc.Update))
	apps.Delete("/:id", h.Uninstall)

	apps.Post("/:id/deploy", fiberutil.ActionItemNested("serverId", "id", "Redeploy started", svc.Deploy))
	apps.Post("/:id/start", fiberutil.ActionItemNested("serverId", "id", "Application started", svc.Start))
	apps.Post("/:id/stop", fiberutil.ActionItemNested("serverId", "id", "Application stopped", svc.Stop))
	apps.Post("/:id/restart", fiberutil.ActionItemNested("serverId", "id", "Application restarted", svc.Restart))

	// Logs uses :appId so the handler can extract it via the helper.
	router.Get("/servers/:serverId/apps/:appId/logs", append(auth, h.Logs)...)

	// Sub-resources: env vars, ports, volumes, domains.
	envVars := router.Group("/servers/:serverId/apps/:appId/env-vars", auth...)
	envVars.Get("/", fiberutil.IndexDoubleNested("serverId", "appId", "Env vars retrieved", svc.ListEnvVars))
	envVars.Post("/", fiberutil.CreateDoubleNested[dto.CreateEnvVarRequest]("serverId", "appId", "Env var created", svc.CreateEnvVar))
	envVars.Delete("/:id", fiberutil.DeleteDoubleNested("serverId", "appId", "id", svc.DeleteEnvVar))

	ports := router.Group("/servers/:serverId/apps/:appId/ports", auth...)
	ports.Get("/", fiberutil.IndexDoubleNested("serverId", "appId", "Ports retrieved", svc.ListPorts))
	ports.Post("/", fiberutil.CreateDoubleNested[dto.CreatePortRequest]("serverId", "appId", "Port created", svc.CreatePort))
	ports.Delete("/:id", fiberutil.DeleteDoubleNested("serverId", "appId", "id", svc.DeletePort))

	volumes := router.Group("/servers/:serverId/apps/:appId/volumes", auth...)
	volumes.Get("/", fiberutil.IndexDoubleNested("serverId", "appId", "Volumes retrieved", svc.ListVolumes))
	volumes.Post("/", fiberutil.CreateDoubleNested[dto.CreateVolumeRequest]("serverId", "appId", "Volume created", svc.CreateVolume))
	volumes.Delete("/:id", fiberutil.DeleteDoubleNested("serverId", "appId", "id", svc.DeleteVolume))

	domains := router.Group("/servers/:serverId/apps/:appId/domains", auth...)
	domains.Get("/", fiberutil.IndexDoubleNested("serverId", "appId", "Domains retrieved", svc.ListDomains))
	domains.Post("/", fiberutil.CreateDoubleNested[dto.CreateDomainRequest]("serverId", "appId", "Domain added", svc.CreateDomain))
	domains.Delete("/:id", fiberutil.DeleteDoubleNested("serverId", "appId", "id", svc.DeleteDomain))
}
