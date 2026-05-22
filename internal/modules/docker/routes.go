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
}
