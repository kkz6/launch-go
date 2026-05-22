package docker

import (
	gofiber "github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/middleware"
	"github.com/kkz6/launch-go/internal/modules/docker/dto"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// RegisterRoutes mounts the docker module's HTTP routes under
// /api/servers/:serverId/docker/...
//
// The route closures use fiberutil's nested-helper combinators so the
// service methods are written once with team-aware signatures (no manual
// teamID extraction in each closure).
func (m *Module) RegisterRoutes(router gofiber.Router, authMiddleware gofiber.Handler) {
	projectSvc := m.newProjectService()
	applicationSvc := m.newApplicationService()

	auth := middleware.Append(
		middleware.AuthenticatedChain(authMiddleware),
		middleware.RequireProvisionedServer("serverId"),
	)

	projects := router.Group("/servers/:serverId/docker/projects", auth...)
	projects.Get(
		"/",
		fiberutil.IndexNested("serverId", "Projects retrieved", projectSvc.ListProjects),
	)
	projects.Post(
		"/",
		fiberutil.CreateNested[dto.CreateProjectRequest]("serverId", "Project created", projectSvc.CreateProject),
	)
	projects.Get(
		"/:id",
		fiberutil.ShowNested("serverId", "id", "Project retrieved", projectSvc.GetProject),
	)
	projects.Patch(
		"/:id",
		fiberutil.UpdateNested[dto.UpdateProjectRequest]("serverId", "id", "Project updated", projectSvc.UpdateProject),
	)
	projects.Delete(
		"/:id",
		fiberutil.DeleteNested("serverId", "id", projectSvc.DeleteProject),
	)

	// Doubly-nested under projects. Grandparent param is serverId so the
	// existing RequireProvisionedServer middleware keeps working; the
	// project lookup happens inside each service method.
	apps := router.Group("/servers/:serverId/docker/projects/:projectId/applications", auth...)
	apps.Get(
		"/",
		fiberutil.IndexDoubleNested("serverId", "projectId", "Applications retrieved", applicationSvc.ListApplications),
	)
	apps.Post(
		"/",
		fiberutil.CreateDoubleNested[dto.CreateApplicationRequest]("serverId", "projectId", "Application created", applicationSvc.CreateApplication),
	)
	apps.Get(
		"/:id",
		fiberutil.ShowDoubleNested("serverId", "projectId", "id", "Application retrieved", applicationSvc.GetApplication),
	)
	apps.Patch(
		"/:id",
		fiberutil.UpdateDoubleNested[dto.UpdateApplicationRequest]("serverId", "projectId", "id", "Application updated", applicationSvc.UpdateApplication),
	)
	apps.Delete(
		"/:id",
		fiberutil.DeleteDoubleNested("serverId", "projectId", "id", applicationSvc.DeleteApplication),
	)
}
