package docker

import (
	gofiber "github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/middleware"
	"github.com/kkz6/launch-go/internal/modules/docker/dto"
	"github.com/kkz6/launch-go/internal/modules/docker/services"
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
	databaseSvc := m.newDatabaseService()
	domainSvc := m.newDomainService()
	envVarSvc := m.newEnvVarService()
	volumeSvc := m.newVolumeService()
	hostSvc := m.newHostInspectService()

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

	// Env-var routes — list / create / update / delete / bulk-set.
	apps.Get("/:id/env-vars", func(c *gofiber.Ctx) error {
		teamID, err := fiberutil.MustGetTeamID(c)
		if err != nil {
			return err
		}
		rows, err := envVarSvc.ListEnvVars(
			c.Context(),
			c.Params("id"),
			c.Params("projectId"),
			c.Params("serverId"),
			teamID,
		)
		if err != nil {
			return err
		}
		return fiberutil.OK(c, "Env vars retrieved", rows)
	})

	apps.Post("/:id/env-vars", func(c *gofiber.Ctx) error {
		teamID, userID, err := fiberutil.MustGetTeamAndUserID(c)
		if err != nil {
			return err
		}
		req, err := fiberutil.MustParseAndValidate[dto.CreateEnvVarRequest](c)
		if err != nil {
			return err
		}
		out, err := envVarSvc.CreateEnvVar(
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
		return fiberutil.Created(c, "Env var added", out)
	})

	apps.Put("/:id/env-vars", func(c *gofiber.Ctx) error {
		teamID, userID, err := fiberutil.MustGetTeamAndUserID(c)
		if err != nil {
			return err
		}
		req, err := fiberutil.MustParseAndValidate[dto.SetEnvVarsRequest](c)
		if err != nil {
			return err
		}
		out, err := envVarSvc.SetEnvVars(
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
		return fiberutil.OK(c, "Env vars saved", out)
	})

	apps.Patch("/:id/env-vars/:envVarId", func(c *gofiber.Ctx) error {
		teamID, userID, err := fiberutil.MustGetTeamAndUserID(c)
		if err != nil {
			return err
		}
		req, err := fiberutil.MustParseAndValidate[dto.UpdateEnvVarRequest](c)
		if err != nil {
			return err
		}
		out, err := envVarSvc.UpdateEnvVar(
			c.Context(),
			c.Params("envVarId"),
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
		return fiberutil.OK(c, "Env var updated", out)
	})

	apps.Delete("/:id/env-vars/:envVarId", func(c *gofiber.Ctx) error {
		teamID, userID, err := fiberutil.MustGetTeamAndUserID(c)
		if err != nil {
			return err
		}
		if err := envVarSvc.DeleteEnvVar(
			c.Context(),
			c.Params("envVarId"),
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

	// Volume routes — list / create / update / delete.
	apps.Get("/:id/volumes", func(c *gofiber.Ctx) error {
		teamID, err := fiberutil.MustGetTeamID(c)
		if err != nil {
			return err
		}
		rows, err := volumeSvc.ListVolumes(
			c.Context(),
			c.Params("id"),
			c.Params("projectId"),
			c.Params("serverId"),
			teamID,
		)
		if err != nil {
			return err
		}
		return fiberutil.OK(c, "Volumes retrieved", rows)
	})

	apps.Post("/:id/volumes", func(c *gofiber.Ctx) error {
		teamID, userID, err := fiberutil.MustGetTeamAndUserID(c)
		if err != nil {
			return err
		}
		req, err := fiberutil.MustParseAndValidate[dto.CreateVolumeRequest](c)
		if err != nil {
			return err
		}
		out, err := volumeSvc.CreateVolume(
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
		return fiberutil.Created(c, "Volume added", out)
	})

	apps.Patch("/:id/volumes/:volumeId", func(c *gofiber.Ctx) error {
		teamID, userID, err := fiberutil.MustGetTeamAndUserID(c)
		if err != nil {
			return err
		}
		req, err := fiberutil.MustParseAndValidate[dto.UpdateVolumeRequest](c)
		if err != nil {
			return err
		}
		out, err := volumeSvc.UpdateVolume(
			c.Context(),
			c.Params("volumeId"),
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
		return fiberutil.OK(c, "Volume updated", out)
	})

	apps.Delete("/:id/volumes/:volumeId", func(c *gofiber.Ctx) error {
		teamID, userID, err := fiberutil.MustGetTeamAndUserID(c)
		if err != nil {
			return err
		}
		if err := volumeSvc.DeleteVolume(
			c.Context(),
			c.Params("volumeId"),
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

	// Managed-database routes. Mostly the same shape as applications/
	// composes; the differentiator is the /lifecycle endpoint that runs
	// start/stop/restart against an existing container.
	databases := router.Group("/servers/:serverId/docker/projects/:projectId/databases", auth...)
	databases.Get(
		"/",
		fiberutil.IndexDoubleNested("serverId", "projectId", "Databases retrieved", databaseSvc.ListDatabases),
	)
	databases.Post(
		"/",
		fiberutil.CreateDoubleNested[dto.CreateDatabaseRequest]("serverId", "projectId", "Database creation queued", databaseSvc.CreateDatabase),
	)
	databases.Delete(
		"/:id",
		fiberutil.DeleteDoubleNested("serverId", "projectId", "id", databaseSvc.DeleteDatabase),
	)

	databases.Get("/:id", func(c *gofiber.Ctx) error {
		teamID, err := fiberutil.MustGetTeamID(c)
		if err != nil {
			return err
		}
		reveal := c.Query("reveal") == "true"
		out, err := databaseSvc.GetDatabase(
			c.Context(),
			c.Params("id"),
			c.Params("projectId"),
			c.Params("serverId"),
			teamID,
			reveal,
		)
		if err != nil {
			return err
		}
		return fiberutil.OK(c, "Database retrieved", out)
	})

	databases.Post("/:id/lifecycle", func(c *gofiber.Ctx) error {
		teamID, userID, err := fiberutil.MustGetTeamAndUserID(c)
		if err != nil {
			return err
		}
		req, err := fiberutil.MustParseAndValidate[dto.DatabaseLifecycleRequest](c)
		if err != nil {
			return err
		}
		out, err := databaseSvc.Lifecycle(
			c.Context(),
			c.Params("id"),
			c.Params("projectId"),
			c.Params("serverId"),
			teamID,
			userID,
			req.Action,
		)
		if err != nil {
			return err
		}
		return fiberutil.OK(c, "Lifecycle action queued", out)
	})

	// Engine catalogue endpoint — the create dialog calls this to know
	// which engines + versions to offer. Returning a static map from
	// the module keeps the UI in sync without an extra config layer.
	//
	// Auth-only (no RequireProvisionedServer) because there's no
	// serverId in the URL — the catalogue is the same for every
	// docker server.
	router.Get(
		"/docker/databases/engines",
		append(middleware.AuthenticatedChain(authMiddleware), func(c *gofiber.Ctx) error {
			return fiberutil.OK(c, "Engine catalogue", services.SupportedDatabaseEngines())
		})...,
	)

	// Server-host diagnostic endpoints — read-only views over
	// `docker ps`, `docker volume ls`, `docker network ls`, and
	// the on-disk Traefik config. Behind the same auth + provisioned-
	// server gate as the workload routes.
	hostGroup := router.Group("/servers/:serverId/docker", auth...)
	hostGroup.Get("/containers", func(c *gofiber.Ctx) error {
		teamID, err := fiberutil.MustGetTeamID(c)
		if err != nil {
			return err
		}
		rows, err := hostSvc.ListContainers(c.Context(), c.Params("serverId"), teamID)
		if err != nil {
			return err
		}
		return fiberutil.OK(c, "Containers retrieved", rows)
	})
	hostGroup.Get("/volumes", func(c *gofiber.Ctx) error {
		teamID, err := fiberutil.MustGetTeamID(c)
		if err != nil {
			return err
		}
		rows, err := hostSvc.ListVolumes(c.Context(), c.Params("serverId"), teamID)
		if err != nil {
			return err
		}
		return fiberutil.OK(c, "Volumes retrieved", rows)
	})
	hostGroup.Get("/networks", func(c *gofiber.Ctx) error {
		teamID, err := fiberutil.MustGetTeamID(c)
		if err != nil {
			return err
		}
		rows, err := hostSvc.ListNetworks(c.Context(), c.Params("serverId"), teamID)
		if err != nil {
			return err
		}
		return fiberutil.OK(c, "Networks retrieved", rows)
	})
	hostGroup.Get("/traefik", func(c *gofiber.Ctx) error {
		teamID, err := fiberutil.MustGetTeamID(c)
		if err != nil {
			return err
		}
		snap, err := hostSvc.GetTraefikSnapshot(c.Context(), c.Params("serverId"), teamID)
		if err != nil {
			return err
		}
		return fiberutil.OK(c, "Traefik snapshot retrieved", snap)
	})
}
