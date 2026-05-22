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
	composeSvc := m.newComposeService()
	domainSvc := m.newDomainService()

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

	// Deployments are triple-nested (server/project/application). The
	// fiberutil double-nested helpers don't cover that depth, so we write
	// the closures by hand here. Same team/userID extraction the helpers
	// would do — kept tight so reviewers can see the whole thing.
	apps.Get("/:id/deployments", func(c *gofiber.Ctx) error {
		teamID, err := fiberutil.MustGetTeamID(c)
		if err != nil {
			return err
		}
		deployments, err := applicationSvc.ListDeployments(
			c.Context(),
			c.Params("id"),
			c.Params("projectId"),
			c.Params("serverId"),
			teamID,
		)
		if err != nil {
			return err
		}
		out := make([]*dto.DeploymentResponse, 0, len(deployments))
		for i := range deployments {
			out = append(out, dto.ToDeploymentResponse(&deployments[i]))
		}
		return fiberutil.OK(c, "Deployments retrieved", out)
	})

	apps.Post("/:id/deploy", func(c *gofiber.Ctx) error {
		teamID, userID, err := fiberutil.MustGetTeamAndUserID(c)
		if err != nil {
			return err
		}
		deployment, err := applicationSvc.Deploy(
			c.Context(),
			c.Params("id"),
			c.Params("projectId"),
			c.Params("serverId"),
			teamID,
			userID,
		)
		if err != nil {
			return err
		}
		return fiberutil.Created(c, "Deployment started", dto.ToDeploymentResponse(deployment))
	})

	// Application-domain CRUD — same triple-nested pattern as deployments.
	apps.Get("/:id/domains", func(c *gofiber.Ctx) error {
		teamID, err := fiberutil.MustGetTeamID(c)
		if err != nil {
			return err
		}
		rows, err := domainSvc.ListDomains(
			c.Context(),
			c.Params("id"),
			c.Params("projectId"),
			c.Params("serverId"),
			teamID,
		)
		if err != nil {
			return err
		}
		return fiberutil.OK(c, "Domains retrieved", rows)
	})

	apps.Post("/:id/domains", func(c *gofiber.Ctx) error {
		teamID, userID, err := fiberutil.MustGetTeamAndUserID(c)
		if err != nil {
			return err
		}
		req, err := fiberutil.MustParseAndValidate[dto.CreateDomainRequest](c)
		if err != nil {
			return err
		}
		out, err := domainSvc.CreateDomain(
			c.Context(),
			c.Params("id"),
			c.Params("projectId"),
			c.Params("serverId"),
			teamID,
			userID,
			req,
		)
		if err != nil {
			return err
		}
		return fiberutil.Created(c, "Domain added", out)
	})

	apps.Patch("/:id/domains/:domainId", func(c *gofiber.Ctx) error {
		teamID, userID, err := fiberutil.MustGetTeamAndUserID(c)
		if err != nil {
			return err
		}
		req, err := fiberutil.MustParseAndValidate[dto.UpdateDomainRequest](c)
		if err != nil {
			return err
		}
		out, err := domainSvc.UpdateDomain(
			c.Context(),
			c.Params("domainId"),
			c.Params("id"),
			c.Params("projectId"),
			c.Params("serverId"),
			teamID,
			userID,
			req,
		)
		if err != nil {
			return err
		}
		return fiberutil.OK(c, "Domain updated", out)
	})

	apps.Delete("/:id/domains/:domainId", func(c *gofiber.Ctx) error {
		teamID, userID, err := fiberutil.MustGetTeamAndUserID(c)
		if err != nil {
			return err
		}
		if err := domainSvc.DeleteDomain(
			c.Context(),
			c.Params("domainId"),
			c.Params("id"),
			c.Params("projectId"),
			c.Params("serverId"),
			teamID,
			userID,
		); err != nil {
			return err
		}
		return fiberutil.NoContent(c)
	})

	// Compose-stack routes mirror the application routes one level down.
	// The fiberutil double-nested helpers handle CRUD; deploy + history
	// are triple-nested closures (same shape as applications).
	composes := router.Group("/servers/:serverId/docker/projects/:projectId/composes", auth...)
	composes.Get(
		"/",
		fiberutil.IndexDoubleNested("serverId", "projectId", "Compose stacks retrieved", composeSvc.ListComposes),
	)
	composes.Post(
		"/",
		fiberutil.CreateDoubleNested[dto.CreateComposeRequest]("serverId", "projectId", "Compose stack created", composeSvc.CreateCompose),
	)
	composes.Get(
		"/:id",
		fiberutil.ShowDoubleNested("serverId", "projectId", "id", "Compose stack retrieved", composeSvc.GetCompose),
	)
	composes.Patch(
		"/:id",
		fiberutil.UpdateDoubleNested[dto.UpdateComposeRequest]("serverId", "projectId", "id", "Compose stack updated", composeSvc.UpdateCompose),
	)
	composes.Delete(
		"/:id",
		fiberutil.DeleteDoubleNested("serverId", "projectId", "id", composeSvc.DeleteCompose),
	)

	composes.Get("/:id/deployments", func(c *gofiber.Ctx) error {
		teamID, err := fiberutil.MustGetTeamID(c)
		if err != nil {
			return err
		}
		rows, err := composeSvc.ListDeployments(
			c.Context(),
			c.Params("id"),
			c.Params("projectId"),
			c.Params("serverId"),
			teamID,
		)
		if err != nil {
			return err
		}
		out := make([]*dto.DeploymentResponse, 0, len(rows))
		for i := range rows {
			out = append(out, dto.ToDeploymentResponse(&rows[i]))
		}
		return fiberutil.OK(c, "Deployments retrieved", out)
	})

	composes.Post("/:id/deploy", func(c *gofiber.Ctx) error {
		teamID, userID, err := fiberutil.MustGetTeamAndUserID(c)
		if err != nil {
			return err
		}
		deployment, err := composeSvc.Deploy(
			c.Context(),
			c.Params("id"),
			c.Params("projectId"),
			c.Params("serverId"),
			teamID,
			userID,
		)
		if err != nil {
			return err
		}
		return fiberutil.Created(c, "Deployment started", dto.ToDeploymentResponse(deployment))
	})
}
