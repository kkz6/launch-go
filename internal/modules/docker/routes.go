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
	projectEnvVarSvc := m.newProjectEnvVarService()
	databaseEnvVarSvc := m.newDatabaseEnvVarService()
	buildSecretSvc := m.newBuildSecretService()
	composeBuildSecretSvc := m.newComposeBuildSecretService()
	volumeSvc := m.newVolumeService()
	hostSvc := m.newHostInspectService()
	scheduleSvc := m.newScheduleService()
	backupSvc := m.newBackupService()
	registryCredSvc := m.newRegistryCredentialService()

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

	// Project-scoped env vars — the source for `${{project.<KEY>}}`
	// references that any workload under the project can use in its
	// own env values. Resolved at deploy/run time by the worker.
	// Nested under /servers/:serverId/docker/projects/:id/env-vars
	// — the project id rides in as :id to match the rest of the
	// project routes' shape.
	projects.Get("/:id/env-vars", func(c *gofiber.Ctx) error {
		teamID, err := fiberutil.MustGetTeamID(c)
		if err != nil {
			return err
		}
		rows, err := projectEnvVarSvc.ListEnvVars(
			c.Context(),
			c.Params("id"),
			c.Params("serverId"),
			teamID,
		)
		if err != nil {
			return err
		}
		return fiberutil.OK(c, "Project env vars retrieved", rows)
	})

	projects.Post("/:id/env-vars", func(c *gofiber.Ctx) error {
		teamID, userID, err := fiberutil.MustGetTeamAndUserID(c)
		if err != nil {
			return err
		}
		req, err := fiberutil.MustParseAndValidate[dto.CreateEnvVarRequest](c)
		if err != nil {
			return err
		}
		out, err := projectEnvVarSvc.CreateEnvVar(
			c.Context(),
			c.Params("id"),
			c.Params("serverId"),
			teamID,
			userID,
			req,
		)
		if err != nil {
			return err
		}
		return fiberutil.Created(c, "Project env var added", out)
	})

	projects.Patch("/:id/env-vars/:envVarId", func(c *gofiber.Ctx) error {
		teamID, userID, err := fiberutil.MustGetTeamAndUserID(c)
		if err != nil {
			return err
		}
		req, err := fiberutil.MustParseAndValidate[dto.UpdateEnvVarRequest](c)
		if err != nil {
			return err
		}
		out, err := projectEnvVarSvc.UpdateEnvVar(
			c.Context(),
			c.Params("envVarId"),
			c.Params("id"),
			c.Params("serverId"),
			teamID,
			userID,
			req,
		)
		if err != nil {
			return err
		}
		return fiberutil.OK(c, "Project env var updated", out)
	})

	projects.Delete("/:id/env-vars/:envVarId", func(c *gofiber.Ctx) error {
		teamID, userID, err := fiberutil.MustGetTeamAndUserID(c)
		if err != nil {
			return err
		}
		if err := projectEnvVarSvc.DeleteEnvVar(
			c.Context(),
			c.Params("envVarId"),
			c.Params("id"),
			c.Params("serverId"),
			teamID,
			userID,
		); err != nil {
			return err
		}
		return fiberutil.NoContent(c)
	})

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
	// Application delete reads `?remove_volumes=true` from the query
	// string. Default false — preserves named volumes the app declared
	// so a fat-fingered Delete doesn't lose persistent data. See the
	// matching closure on the compose DELETE for the same shape.
	apps.Delete("/:id", func(c *gofiber.Ctx) error {
		teamID, userID, err := fiberutil.MustGetTeamAndUserID(c)
		if err != nil {
			return err
		}
		removeVolumes := c.Query("remove_volumes") == "true" || c.Query("remove_volumes") == "1"
		if err := applicationSvc.DeleteApplication(
			c.Context(),
			c.Params("id"),
			c.Params("projectId"),
			c.Params("serverId"),
			teamID, userID, removeVolumes,
		); err != nil {
			return err
		}
		return fiberutil.NoContent(c)
	})

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

	// GitHub Actions detail-page actions (slice I).
	//   rotate-token: mint a fresh deploy token + push the new secret
	//                 to the repo via gha:bootstrap_workflow
	//   resync:       re-render + re-PUT the workflow file only
	//   disable:      flip back to build_location=server + clear local
	//                 GHA state. GitHub-side cleanup (delete secret +
	//                 vars, orphan the workflow file) is a follow-up.
	apps.Post("/:id/gha/rotate-token", func(c *gofiber.Ctx) error {
		teamID, userID, err := fiberutil.MustGetTeamAndUserID(c)
		if err != nil {
			return err
		}
		resp, err := applicationSvc.RotateGHAToken(
			c.Context(), c.Params("id"), c.Params("projectId"), c.Params("serverId"), teamID, userID,
		)
		if err != nil {
			return err
		}
		return fiberutil.OK(c, "Token rotation queued", resp)
	})
	apps.Post("/:id/gha/resync", func(c *gofiber.Ctx) error {
		teamID, userID, err := fiberutil.MustGetTeamAndUserID(c)
		if err != nil {
			return err
		}
		if err := applicationSvc.ResyncGHA(
			c.Context(), c.Params("id"), c.Params("projectId"), c.Params("serverId"), teamID, userID,
		); err != nil {
			return err
		}
		return fiberutil.OK(c, "Workflow re-sync queued", nil)
	})
	apps.Post("/:id/gha/disable", func(c *gofiber.Ctx) error {
		teamID, userID, err := fiberutil.MustGetTeamAndUserID(c)
		if err != nil {
			return err
		}
		if err := applicationSvc.DisableGHA(
			c.Context(), c.Params("id"), c.Params("projectId"), c.Params("serverId"), teamID, userID,
		); err != nil {
			return err
		}
		return fiberutil.OK(c, "GitHub Actions builds disabled", nil)
	})

	// Reload / Stop / Start map to docker restart / stop / start on
	// the running container. We expose them as separate POST verbs
	// (instead of one /:id/lifecycle with an `action` body field)
	// because each has a distinct toast message + status badge mapping
	// — keeping the actions on the URL means the access log is also
	// self-documenting.
	//
	// All three share applicationSvc.Lifecycle under the hood; the
	// closure just plugs the verb in. Rebuild lives on /deploy
	// because force-pull semantics belong to the deploy pipeline.
	for _, verb := range []string{"reload", "stop", "start"} {
		// reload is the UI label; backend action is "restart" (the
		// dokploy convention — "reload" reads better in the Actions
		// dropdown than "restart").
		action := verb
		if verb == "reload" {
			action = "restart"
		}
		apps.Post("/:id/"+verb, func(c *gofiber.Ctx) error {
			teamID, userID, err := fiberutil.MustGetTeamAndUserID(c)
			if err != nil {
				return err
			}
			if err := applicationSvc.Lifecycle(
				c.Context(),
				c.Params("id"),
				c.Params("projectId"),
				c.Params("serverId"),
				teamID,
				userID,
				action,
			); err != nil {
				return err
			}
			return fiberutil.OK(c, "Action enqueued", map[string]string{"action": action})
		})
	}

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

	// Validate DNS — resolves the domain hostname and checks that
	// the A record points at the docker server's public IP. Read-
	// only, no side effects; the frontend's "Validate DNS" pill
	// calls this and surfaces the result as a toast.
	apps.Get("/:id/domains/:domainId/validate-dns", func(c *gofiber.Ctx) error {
		teamID, err := fiberutil.MustGetTeamID(c)
		if err != nil {
			return err
		}
		out, err := domainSvc.ValidateDNS(
			c.Context(),
			c.Params("domainId"),
			c.Params("id"),
			c.Params("projectId"),
			c.Params("serverId"),
			teamID,
		)
		if err != nil {
			return err
		}
		return fiberutil.OK(c, "DNS validation", out)
	})

	// Redirect routes — list / create / update / delete. Backed by
	// build_config.redirects (no table); mirrors the PHP-site shape
	// (from / to / type) so the Redirects subtab can reuse the same
	// DataTable + dialog the SitesRedirects subtab uses.
	apps.Get("/:id/redirects", func(c *gofiber.Ctx) error {
		teamID, err := fiberutil.MustGetTeamID(c)
		if err != nil {
			return err
		}
		rows, err := applicationSvc.ListRedirects(
			c.Context(),
			c.Params("id"),
			c.Params("projectId"),
			c.Params("serverId"),
			teamID,
		)
		if err != nil {
			return err
		}
		return fiberutil.OK(c, "Redirects retrieved", rows)
	})

	apps.Post("/:id/redirects", func(c *gofiber.Ctx) error {
		teamID, userID, err := fiberutil.MustGetTeamAndUserID(c)
		if err != nil {
			return err
		}
		req, err := fiberutil.MustParseAndValidate[dto.CreateApplicationRedirectRequest](c)
		if err != nil {
			return err
		}
		out, err := applicationSvc.CreateRedirect(
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
		return fiberutil.Created(c, "Redirect added", out)
	})

	apps.Patch("/:id/redirects/:redirectId", func(c *gofiber.Ctx) error {
		teamID, userID, err := fiberutil.MustGetTeamAndUserID(c)
		if err != nil {
			return err
		}
		req, err := fiberutil.MustParseAndValidate[dto.UpdateApplicationRedirectRequest](c)
		if err != nil {
			return err
		}
		out, err := applicationSvc.UpdateRedirect(
			c.Context(),
			c.Params("id"),
			c.Params("projectId"),
			c.Params("serverId"),
			teamID,
			userID,
			c.Params("redirectId"),
			req,
		)
		if err != nil {
			return err
		}
		return fiberutil.OK(c, "Redirect updated", out)
	})

	apps.Delete("/:id/redirects/:redirectId", func(c *gofiber.Ctx) error {
		teamID, userID, err := fiberutil.MustGetTeamAndUserID(c)
		if err != nil {
			return err
		}
		if err := applicationSvc.DeleteRedirect(
			c.Context(),
			c.Params("id"),
			c.Params("projectId"),
			c.Params("serverId"),
			teamID,
			userID,
			c.Params("redirectId"),
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

	// Build-time secrets — values mounted into `docker build` via
	// --mount=type=secret. Separate from env-vars: build-time vs
	// runtime, and write-only (the value is never returned in any
	// response). When the application is GHA-backed the service also
	// triggers a workflow re-sync so the YAML's `secrets:` block + the
	// LAUNCH_BUILD_<NAME> repo secrets stay in lock-step with this list.
	apps.Get("/:id/build-secrets", func(c *gofiber.Ctx) error {
		teamID, err := fiberutil.MustGetTeamID(c)
		if err != nil {
			return err
		}
		rows, err := buildSecretSvc.ListBuildSecrets(
			c.Context(),
			c.Params("id"),
			c.Params("projectId"),
			c.Params("serverId"),
			teamID,
		)
		if err != nil {
			return err
		}
		return fiberutil.OK(c, "Build secrets retrieved", rows)
	})

	apps.Post("/:id/build-secrets", func(c *gofiber.Ctx) error {
		teamID, userID, err := fiberutil.MustGetTeamAndUserID(c)
		if err != nil {
			return err
		}
		req, err := fiberutil.MustParseAndValidate[dto.CreateBuildSecretRequest](c)
		if err != nil {
			return err
		}
		out, err := buildSecretSvc.CreateBuildSecret(
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
		return fiberutil.Created(c, "Build secret added", out)
	})

	apps.Patch("/:id/build-secrets/:buildSecretId", func(c *gofiber.Ctx) error {
		teamID, userID, err := fiberutil.MustGetTeamAndUserID(c)
		if err != nil {
			return err
		}
		req, err := fiberutil.MustParseAndValidate[dto.UpdateBuildSecretRequest](c)
		if err != nil {
			return err
		}
		out, err := buildSecretSvc.UpdateBuildSecret(
			c.Context(),
			c.Params("buildSecretId"),
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
		return fiberutil.OK(c, "Build secret updated", out)
	})

	apps.Delete("/:id/build-secrets/:buildSecretId", func(c *gofiber.Ctx) error {
		teamID, userID, err := fiberutil.MustGetTeamAndUserID(c)
		if err != nil {
			return err
		}
		if err := buildSecretSvc.DeleteBuildSecret(
			c.Context(),
			c.Params("buildSecretId"),
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

	// Schedule routes — list / create / update / delete.
	apps.Get("/:id/schedules", func(c *gofiber.Ctx) error {
		teamID, err := fiberutil.MustGetTeamID(c)
		if err != nil {
			return err
		}
		rows, err := scheduleSvc.ListSchedules(
			c.Context(),
			c.Params("id"),
			c.Params("projectId"),
			c.Params("serverId"),
			teamID,
		)
		if err != nil {
			return err
		}
		return fiberutil.OK(c, "Schedules retrieved", rows)
	})
	apps.Post("/:id/schedules", func(c *gofiber.Ctx) error {
		teamID, userID, err := fiberutil.MustGetTeamAndUserID(c)
		if err != nil {
			return err
		}
		req, err := fiberutil.MustParseAndValidate[dto.CreateScheduleRequest](c)
		if err != nil {
			return err
		}
		out, err := scheduleSvc.CreateSchedule(
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
		return fiberutil.Created(c, "Schedule added", out)
	})
	apps.Patch("/:id/schedules/:scheduleId", func(c *gofiber.Ctx) error {
		teamID, userID, err := fiberutil.MustGetTeamAndUserID(c)
		if err != nil {
			return err
		}
		req, err := fiberutil.MustParseAndValidate[dto.UpdateScheduleRequest](c)
		if err != nil {
			return err
		}
		out, err := scheduleSvc.UpdateSchedule(
			c.Context(),
			c.Params("scheduleId"),
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
		return fiberutil.OK(c, "Schedule updated", out)
	})
	apps.Delete("/:id/schedules/:scheduleId", func(c *gofiber.Ctx) error {
		teamID, userID, err := fiberutil.MustGetTeamAndUserID(c)
		if err != nil {
			return err
		}
		if err := scheduleSvc.DeleteSchedule(
			c.Context(),
			c.Params("scheduleId"),
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

	// Per-application Traefik dynamic-config file. Read returns
	// the body of /etc/launch/traefik/dynamic/<project>-<app>.yml
	// as it is on the host RIGHT NOW; write overwrites it via the
	// same path the deploy task uses on a domain attach. Mirrors
	// dokploy's Advanced → Traefik card — operator can spot-check or
	// override the generated routes/services without touching the
	// server-level dynamic dir. Hand-edits SURVIVE the next deploy as
	// long as the YAML is still valid (the deploy task rewrites the
	// file from domain rows, so manual edits are lost when domains
	// change — the card's UI copy calls this out).
	apps.Get("/:id/traefik-config", func(c *gofiber.Ctx) error {
		teamID, err := fiberutil.MustGetTeamID(c)
		if err != nil {
			return err
		}
		out, err := hostSvc.GetApplicationTraefikConfig(
			c.Context(),
			c.Params("id"),
			c.Params("projectId"),
			c.Params("serverId"),
			teamID,
		)
		if err != nil {
			return err
		}
		return fiberutil.OK(c, "Traefik config retrieved", out)
	})
	apps.Patch("/:id/traefik-config", func(c *gofiber.Ctx) error {
		teamID, err := fiberutil.MustGetTeamID(c)
		if err != nil {
			return err
		}
		req, err := fiberutil.MustParseAndValidate[dto.UpdateApplicationTraefikConfigRequest](c)
		if err != nil {
			return err
		}
		out, err := hostSvc.UpdateApplicationTraefikConfig(
			c.Context(),
			c.Params("id"),
			c.Params("projectId"),
			c.Params("serverId"),
			teamID,
			req.Content,
		)
		if err != nil {
			// Surface validation errors (size cap, etc.) as 400s — the
			// underlying WriteTraefikDynamicFile distinguishes between
			// "your input is bad" and "the host is broken" with the
			// error wording.
			return fiberutil.BadRequest(err.Error())
		}
		return fiberutil.OK(c, "Traefik config updated", out)
	})

	// Advanced runtime settings — merged into build_config and applied on
	// the next deploy.
	apps.Patch("/:id/advanced", func(c *gofiber.Ctx) error {
		teamID, userID, err := fiberutil.MustGetTeamAndUserID(c)
		if err != nil {
			return err
		}
		req, err := fiberutil.MustParseAndValidate[dto.UpdateAdvancedRequest](c)
		if err != nil {
			return err
		}
		out, err := applicationSvc.UpdateAdvanced(
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
		return fiberutil.OK(c, "Advanced settings updated", out)
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
	// Compose delete reads `?remove_volumes=true` from the query
	// string and threads it into the service so the user can opt in
	// to wiping named volumes alongside the containers. Default false
	// (preserves data); the UI surfaces a checkbox on the Delete
	// confirmation dialog. We can't use fiberutil.DeleteDoubleNested
	// because that helper doesn't pass the query through.
	composes.Delete("/:id", func(c *gofiber.Ctx) error {
		teamID, userID, err := fiberutil.MustGetTeamAndUserID(c)
		if err != nil {
			return err
		}
		removeVolumes := c.Query("remove_volumes") == "true" || c.Query("remove_volumes") == "1"
		if err := composeSvc.DeleteCompose(
			c.Context(),
			c.Params("id"),
			c.Params("projectId"),
			c.Params("serverId"),
			teamID, userID, removeVolumes,
		); err != nil {
			return err
		}
		return fiberutil.NoContent(c)
	})

	// Service names belonging to this compose stack — drives the
	// container picker on the Logs subtab. SSHes to the host and runs
	// `docker compose ps --services`. Empty list when the stack has
	// never been deployed; the picker falls back to "all services".
	composes.Get("/:id/services", func(c *gofiber.Ctx) error {
		teamID, err := fiberutil.MustGetTeamID(c)
		if err != nil {
			return err
		}
		services, err := composeSvc.ListServices(
			c.Context(),
			c.Params("id"),
			c.Params("projectId"),
			c.Params("serverId"),
			teamID,
		)
		if err != nil {
			return err
		}
		return fiberutil.OK(c, "Compose services retrieved", services)
	})

	// Default-run-command preview — the Advanced subtab shows this in
	// its "Default Command (...)" hint when the operator hasn't set
	// a per-stack override. Wraps the same renderer the deploy job
	// uses so the hint and the script can't disagree.
	composes.Get("/:id/default-command", func(c *gofiber.Ctx) error {
		teamID, err := fiberutil.MustGetTeamID(c)
		if err != nil {
			return err
		}
		cmd, err := composeSvc.GetDefaultRunCommand(
			c.Context(),
			c.Params("id"),
			c.Params("projectId"),
			c.Params("serverId"),
			teamID,
		)
		if err != nil {
			return err
		}
		return fiberutil.OK(c, "Default command retrieved", map[string]any{
			"command": cmd,
		})
	})

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

	// Reload: recreate the stack with the current .env and no rebuild
	// (reuse on-host images) so saved env changes apply fast. Build-time
	// changes still go through /deploy.
	composes.Post("/:id/reload", func(c *gofiber.Ctx) error {
		teamID, userID, err := fiberutil.MustGetTeamAndUserID(c)
		if err != nil {
			return err
		}
		deployment, err := composeSvc.Reload(
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
		return fiberutil.Created(c, "Reload started", dto.ToDeploymentResponse(deployment))
	})

	// GitHub Actions detail-page actions for composes — mirror of the
	// application trio above. Same semantics, just routed at the
	// compose's URL.
	composes.Post("/:id/gha/rotate-token", func(c *gofiber.Ctx) error {
		teamID, userID, err := fiberutil.MustGetTeamAndUserID(c)
		if err != nil {
			return err
		}
		resp, err := composeSvc.RotateGHAToken(
			c.Context(), c.Params("id"), c.Params("projectId"), c.Params("serverId"), teamID, userID,
		)
		if err != nil {
			return err
		}
		return fiberutil.OK(c, "Token rotation queued", resp)
	})
	composes.Post("/:id/gha/resync", func(c *gofiber.Ctx) error {
		teamID, userID, err := fiberutil.MustGetTeamAndUserID(c)
		if err != nil {
			return err
		}
		if err := composeSvc.ResyncGHA(
			c.Context(), c.Params("id"), c.Params("projectId"), c.Params("serverId"), teamID, userID,
		); err != nil {
			return err
		}
		return fiberutil.OK(c, "Workflow re-sync queued", nil)
	})
	composes.Post("/:id/gha/disable", func(c *gofiber.Ctx) error {
		teamID, userID, err := fiberutil.MustGetTeamAndUserID(c)
		if err != nil {
			return err
		}
		if err := composeSvc.DisableGHA(
			c.Context(), c.Params("id"), c.Params("projectId"), c.Params("serverId"), teamID, userID,
		); err != nil {
			return err
		}
		return fiberutil.OK(c, "GitHub Actions builds disabled", nil)
	})

	// Compose build-time secrets — same write-only semantics as the
	// application build-secret routes above. One secret name is
	// available to every service in the stack that references it
	// from its own Dockerfile via id=NAME under --mount=type=secret.
	composes.Get("/:id/build-secrets", func(c *gofiber.Ctx) error {
		teamID, err := fiberutil.MustGetTeamID(c)
		if err != nil {
			return err
		}
		rows, err := composeBuildSecretSvc.ListBuildSecrets(
			c.Context(),
			c.Params("id"),
			c.Params("projectId"),
			c.Params("serverId"),
			teamID,
		)
		if err != nil {
			return err
		}
		return fiberutil.OK(c, "Build secrets retrieved", rows)
	})

	composes.Post("/:id/build-secrets", func(c *gofiber.Ctx) error {
		teamID, userID, err := fiberutil.MustGetTeamAndUserID(c)
		if err != nil {
			return err
		}
		req, err := fiberutil.MustParseAndValidate[dto.CreateBuildSecretRequest](c)
		if err != nil {
			return err
		}
		out, err := composeBuildSecretSvc.CreateBuildSecret(
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
		return fiberutil.Created(c, "Build secret added", out)
	})

	composes.Patch("/:id/build-secrets/:buildSecretId", func(c *gofiber.Ctx) error {
		teamID, userID, err := fiberutil.MustGetTeamAndUserID(c)
		if err != nil {
			return err
		}
		req, err := fiberutil.MustParseAndValidate[dto.UpdateBuildSecretRequest](c)
		if err != nil {
			return err
		}
		out, err := composeBuildSecretSvc.UpdateBuildSecret(
			c.Context(),
			c.Params("buildSecretId"),
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
		return fiberutil.OK(c, "Build secret updated", out)
	})

	composes.Delete("/:id/build-secrets/:buildSecretId", func(c *gofiber.Ctx) error {
		teamID, userID, err := fiberutil.MustGetTeamAndUserID(c)
		if err != nil {
			return err
		}
		if err := composeBuildSecretSvc.DeleteBuildSecret(
			c.Context(),
			c.Params("buildSecretId"),
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

	// Compose volume routes — list / create / update / delete. Shares
	// the same VolumeService + repository as the application path; only
	// the owner column differs (`compose_id` vs `application_id`).
	// File-type rows are materialized to `${STACK_DIR}/files/<file_path>`
	// before `docker compose up` runs (see tasks/deploy_compose.go) so
	// the YAML can reference them via `./files/<file_path>`. Bind /
	// volume rows are informational — the operator wires them into the
	// YAML themselves; we don't rewrite docker-compose.yml.
	composes.Get("/:id/volumes", func(c *gofiber.Ctx) error {
		teamID, err := fiberutil.MustGetTeamID(c)
		if err != nil {
			return err
		}
		rows, err := volumeSvc.ListComposeVolumes(
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

	composes.Post("/:id/volumes", func(c *gofiber.Ctx) error {
		teamID, userID, err := fiberutil.MustGetTeamAndUserID(c)
		if err != nil {
			return err
		}
		req, err := fiberutil.MustParseAndValidate[dto.CreateVolumeRequest](c)
		if err != nil {
			return err
		}
		out, err := volumeSvc.CreateComposeVolume(
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

	composes.Patch("/:id/volumes/:volumeId", func(c *gofiber.Ctx) error {
		teamID, userID, err := fiberutil.MustGetTeamAndUserID(c)
		if err != nil {
			return err
		}
		req, err := fiberutil.MustParseAndValidate[dto.UpdateVolumeRequest](c)
		if err != nil {
			return err
		}
		out, err := volumeSvc.UpdateComposeVolume(
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

	composes.Delete("/:id/volumes/:volumeId", func(c *gofiber.Ctx) error {
		teamID, userID, err := fiberutil.MustGetTeamAndUserID(c)
		if err != nil {
			return err
		}
		if err := volumeSvc.DeleteComposeVolume(
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

	// Compose Domains — same shape as the application Domains routes
	// (list/create/update/delete + validate-dns). Compose domains
	// require service_name + container_port on Create because the
	// Traefik renderer needs both to target the right container.
	composes.Get("/:id/domains", func(c *gofiber.Ctx) error {
		teamID, err := fiberutil.MustGetTeamID(c)
		if err != nil {
			return err
		}
		rows, err := domainSvc.ListComposeDomains(
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

	composes.Post("/:id/domains", func(c *gofiber.Ctx) error {
		teamID, userID, err := fiberutil.MustGetTeamAndUserID(c)
		if err != nil {
			return err
		}
		req, err := fiberutil.MustParseAndValidate[dto.CreateDomainRequest](c)
		if err != nil {
			return err
		}
		out, err := domainSvc.CreateComposeDomain(
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

	composes.Patch("/:id/domains/:domainId", func(c *gofiber.Ctx) error {
		teamID, userID, err := fiberutil.MustGetTeamAndUserID(c)
		if err != nil {
			return err
		}
		req, err := fiberutil.MustParseAndValidate[dto.UpdateDomainRequest](c)
		if err != nil {
			return err
		}
		out, err := domainSvc.UpdateComposeDomain(
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

	composes.Delete("/:id/domains/:domainId", func(c *gofiber.Ctx) error {
		teamID, userID, err := fiberutil.MustGetTeamAndUserID(c)
		if err != nil {
			return err
		}
		if err := domainSvc.DeleteComposeDomain(
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

	composes.Get("/:id/domains/:domainId/validate-dns", func(c *gofiber.Ctx) error {
		teamID, err := fiberutil.MustGetTeamID(c)
		if err != nil {
			return err
		}
		out, err := domainSvc.ValidateComposeDNS(
			c.Context(),
			c.Params("domainId"),
			c.Params("id"),
			c.Params("projectId"),
			c.Params("serverId"),
			teamID,
		)
		if err != nil {
			return err
		}
		return fiberutil.OK(c, "DNS validation", out)
	})

	// Per-compose Traefik dynamic-config card. Reads/writes
	// /etc/launch/traefik/dynamic/compose-<project>-<compose>.yml —
	// the file the SyncComposeTraefikConfig job populates from domain
	// rows on every mutation. Hand-edits survive until the next
	// domain change clobbers them; the UI copy spells that out.
	composes.Get("/:id/traefik-config", func(c *gofiber.Ctx) error {
		teamID, err := fiberutil.MustGetTeamID(c)
		if err != nil {
			return err
		}
		out, err := hostSvc.GetComposeTraefikConfig(
			c.Context(),
			c.Params("id"),
			c.Params("projectId"),
			c.Params("serverId"),
			teamID,
		)
		if err != nil {
			return err
		}
		return fiberutil.OK(c, "Traefik config retrieved", out)
	})
	composes.Patch("/:id/traefik-config", func(c *gofiber.Ctx) error {
		teamID, err := fiberutil.MustGetTeamID(c)
		if err != nil {
			return err
		}
		req, err := fiberutil.MustParseAndValidate[dto.UpdateApplicationTraefikConfigRequest](c)
		if err != nil {
			return err
		}
		out, err := hostSvc.UpdateComposeTraefikConfig(
			c.Context(),
			c.Params("id"),
			c.Params("projectId"),
			c.Params("serverId"),
			teamID,
			req.Content,
		)
		if err != nil {
			return fiberutil.BadRequest(err.Error())
		}
		return fiberutil.OK(c, "Traefik config updated", out)
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
		// Database delete reads `?remove_volumes=true` and threads it
		// into the rm lifecycle action. False (default) keeps the named
		// data volume; true wipes it so the database starts fresh on
		// recreate. Same shape as the application + compose routes.
		func(c *gofiber.Ctx) error {
			teamID, userID, err := fiberutil.MustGetTeamAndUserID(c)
			if err != nil {
				return err
			}
			removeVolumes := c.Query("remove_volumes") == "true" || c.Query("remove_volumes") == "1"
			if err := databaseSvc.DeleteDatabase(
				c.Context(),
				c.Params("id"),
				c.Params("projectId"),
				c.Params("serverId"),
				teamID, userID, removeVolumes,
			); err != nil {
				return err
			}
			return fiberutil.NoContent(c)
		},
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

	// Lifecycle history for a database — same shape as application +
	// compose deploys. Reads from docker_deployments where target_type =
	// "database" (see DatabaseService.ListDeployments).
	databases.Get("/:id/deployments", func(c *gofiber.Ctx) error {
		teamID, err := fiberutil.MustGetTeamID(c)
		if err != nil {
			return err
		}
		rows, err := databaseSvc.ListDeployments(
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

	// Toggle the database's external port. Enabled=true with a port
	// (defaulting to the engine's standard) puts a -p mapping on the
	// container; Enabled=false clears it. RunDatabaseJob's idempotent
	// script recreates the container with the new state.
	databases.Post("/:id/expose", func(c *gofiber.Ctx) error {
		teamID, userID, err := fiberutil.MustGetTeamAndUserID(c)
		if err != nil {
			return err
		}
		req, err := fiberutil.MustParseAndValidate[dto.SetDatabaseExposeRequest](c)
		if err != nil {
			return err
		}
		out, err := databaseSvc.SetExposeExternal(
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
		return fiberutil.OK(c, "Expose setting updated", out)
	})

	// Rebuild Database — Danger Zone action. Stops the container, wipes
	// the named data volume, then starts it again so the engine
	// reinitialises from scratch with the same image + credentials. No
	// request body: the URL fully identifies the target.
	databases.Post("/:id/rebuild", func(c *gofiber.Ctx) error {
		teamID, userID, err := fiberutil.MustGetTeamAndUserID(c)
		if err != nil {
			return err
		}
		out, err := databaseSvc.RebuildDatabase(
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
		return fiberutil.OK(c, "Database rebuild queued", out)
	})

	// Database advanced settings — restart policy + resource limits
	// (CPU / memory + reservations). Persists into build_config and
	// dispatches a `docker update` over SSH so the change applies to
	// the running container immediately.
	databases.Patch("/:id/advanced", func(c *gofiber.Ctx) error {
		teamID, userID, err := fiberutil.MustGetTeamAndUserID(c)
		if err != nil {
			return err
		}
		req, err := fiberutil.MustParseAndValidate[dto.UpdateDatabaseAdvancedRequest](c)
		if err != nil {
			return err
		}
		out, err := databaseSvc.UpdateDatabaseAdvanced(
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
		return fiberutil.OK(c, "Database advanced settings updated", out)
	})

	// Database env-var CRUD — user-added env vars layered on top of
	// the auto-generated engine credentials. Same shape as the
	// application env-var routes; key/value/is_secret semantics.
	// Values may reference `${{project.<KEY>}}` — the run-database
	// worker resolves them at docker-run time.
	databases.Get("/:id/env-vars", func(c *gofiber.Ctx) error {
		teamID, err := fiberutil.MustGetTeamID(c)
		if err != nil {
			return err
		}
		rows, err := databaseEnvVarSvc.ListEnvVars(
			c.Context(),
			c.Params("id"),
			c.Params("projectId"),
			c.Params("serverId"),
			teamID,
		)
		if err != nil {
			return err
		}
		return fiberutil.OK(c, "Database env vars retrieved", rows)
	})

	databases.Post("/:id/env-vars", func(c *gofiber.Ctx) error {
		teamID, userID, err := fiberutil.MustGetTeamAndUserID(c)
		if err != nil {
			return err
		}
		req, err := fiberutil.MustParseAndValidate[dto.CreateEnvVarRequest](c)
		if err != nil {
			return err
		}
		out, err := databaseEnvVarSvc.CreateEnvVar(
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
		return fiberutil.Created(c, "Database env var added", out)
	})

	databases.Patch("/:id/env-vars/:envVarId", func(c *gofiber.Ctx) error {
		teamID, userID, err := fiberutil.MustGetTeamAndUserID(c)
		if err != nil {
			return err
		}
		req, err := fiberutil.MustParseAndValidate[dto.UpdateEnvVarRequest](c)
		if err != nil {
			return err
		}
		out, err := databaseEnvVarSvc.UpdateEnvVar(
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
		return fiberutil.OK(c, "Database env var updated", out)
	})

	databases.Delete("/:id/env-vars/:envVarId", func(c *gofiber.Ctx) error {
		teamID, userID, err := fiberutil.MustGetTeamAndUserID(c)
		if err != nil {
			return err
		}
		if err := databaseEnvVarSvc.DeleteEnvVar(
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

	// Database backup routes. /backup is singleton (one config per
	// database); /backup/runs is the history list; /backup/run is the
	// "run now" entrypoint; /backup/restore replays a past run.
	databases.Get("/:id/backup", func(c *gofiber.Ctx) error {
		teamID, err := fiberutil.MustGetTeamID(c)
		if err != nil {
			return err
		}
		out, err := backupSvc.GetBackup(
			c.Context(),
			c.Params("id"),
			c.Params("projectId"),
			c.Params("serverId"),
			teamID,
		)
		if err != nil {
			return err
		}
		return fiberutil.OK(c, "Backup config retrieved", out)
	})
	databases.Put("/:id/backup", func(c *gofiber.Ctx) error {
		teamID, userID, err := fiberutil.MustGetTeamAndUserID(c)
		if err != nil {
			return err
		}
		req, err := fiberutil.MustParseAndValidate[dto.ConfigureBackupRequest](c)
		if err != nil {
			return err
		}
		out, err := backupSvc.ConfigureBackup(
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
		return fiberutil.OK(c, "Backup config saved", out)
	})
	databases.Delete("/:id/backup", func(c *gofiber.Ctx) error {
		teamID, userID, err := fiberutil.MustGetTeamAndUserID(c)
		if err != nil {
			return err
		}
		if err := backupSvc.DeleteBackup(
			c.Context(),
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
	databases.Get("/:id/backup/runs", func(c *gofiber.Ctx) error {
		teamID, err := fiberutil.MustGetTeamID(c)
		if err != nil {
			return err
		}
		rows, err := backupSvc.ListRuns(
			c.Context(),
			c.Params("id"),
			c.Params("projectId"),
			c.Params("serverId"),
			teamID,
		)
		if err != nil {
			return err
		}
		return fiberutil.OK(c, "Backup runs retrieved", rows)
	})
	databases.Post("/:id/backup/run", func(c *gofiber.Ctx) error {
		teamID, userID, err := fiberutil.MustGetTeamAndUserID(c)
		if err != nil {
			return err
		}
		out, err := backupSvc.RunNow(
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
		return fiberutil.Created(c, "Backup run started", out)
	})
	databases.Post("/:id/backup/restore", func(c *gofiber.Ctx) error {
		teamID, userID, err := fiberutil.MustGetTeamAndUserID(c)
		if err != nil {
			return err
		}
		req, err := fiberutil.MustParseAndValidate[dto.RestoreBackupRequest](c)
		if err != nil {
			return err
		}
		if err := backupSvc.Restore(
			c.Context(),
			c.Params("id"),
			c.Params("projectId"),
			c.Params("serverId"),
			teamID,
			userID,
			req,
		); err != nil {
			return err
		}
		return fiberutil.OK(c, "Restore complete", nil)
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

	// Server-level docker database listing. Spans every project on
	// the server so the restore dialog can show "all candidate target
	// databases" in one dropdown (a typical restore points prod →
	// staging where those rows usually live in DIFFERENT projects).
	router.Get(
		"/servers/:serverId/docker/databases",
		append(auth, func(c *gofiber.Ctx) error {
			teamID, err := fiberutil.MustGetTeamID(c)
			if err != nil {
				return err
			}
			rows, err := m.newDatabaseService().ListDatabasesForServer(
				c.Context(), c.Params("serverId"), teamID,
			)
			if err != nil {
				return err
			}
			return fiberutil.OK(c, "Server databases retrieved", rows)
		})...,
	)

	// Purge orphaned compose resources — queues the same label-based
	// RemoveComposeJob as the normal delete flow, but driven by project
	// name alone (no live compose row needed). Intended for cleanup of
	// stacks that were deleted while the old broken teardown script was
	// in place, leaving containers running on the host.
	//
	// Separate from the compose CRUD group because no projectId is
	// needed — the caller supplies the project name directly (the
	// com.docker.compose.project label value). Sits at the server level
	// so it can't be confused with the per-compose delete endpoint.
	hostGroup2 := router.Group("/servers/:serverId/docker", auth...)
	hostGroup2.Post("/purge-compose-resources", func(c *gofiber.Ctx) error {
		teamID, err := fiberutil.MustGetTeamID(c)
		if err != nil {
			return err
		}
		req, err := fiberutil.MustParseAndValidate[dto.PurgeComposeResourcesRequest](c)
		if err != nil {
			return err
		}
		if err := composeSvc.PurgeComposeResources(
			c.Context(),
			c.Params("serverId"),
			teamID,
			req,
		); err != nil {
			return err
		}
		return fiberutil.OK(c, "Cleanup job queued", nil)
	})

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
	// Per-container inspect — fuller picture than `docker ps` gives
	// us (state, health, mounts, networks, resources). The frontend
	// opens this in a status dialog when the user clicks a container's
	// state badge.
	hostGroup.Get("/containers/:containerId/inspect", func(c *gofiber.Ctx) error {
		teamID, err := fiberutil.MustGetTeamID(c)
		if err != nil {
			return err
		}
		info, err := hostSvc.InspectContainer(
			c.Context(),
			c.Params("serverId"),
			teamID,
			c.Params("containerId"),
		)
		if err != nil {
			return err
		}
		return fiberutil.OK(c, "Container inspected", info)
	})
	// Host-level /volumes endpoint intentionally not registered: the
	// UI tab was removed because per-app volume management lives on
	// the Application → Volumes subtab (bind / volume / file mount
	// picker). A flat host-wide list of docker volumes invites
	// accidental cleanup of volumes belonging to running apps and
	// doesn't add operational value beyond the per-app view.
	// `HostInspectService.ListVolumes` stays for future use.
	//
	// Networks endpoint intentionally not registered: the UI tab was
	// removed (Launch manages the launch-network overlay; users
	// don't create custom networks via the UI). ListNetworks /
	// NetworkInfo stay in HostInspectService for future use and
	// because the docker.compose deploy task uses the same SSH
	// path to discover network attachments.
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

	// Edit a dynamic Traefik config file. The filename is in the URL
	// rather than the body so the route reads as a normal resource and
	// the path-validation helper has the canonical input. The body is
	// just the YAML contents.
	hostGroup.Put("/traefik/files/:filename", func(c *gofiber.Ctx) error {
		teamID, err := fiberutil.MustGetTeamID(c)
		if err != nil {
			return err
		}
		body := struct {
			Contents string `json:"contents" validate:"required,max=262144"`
		}{}
		if err := c.BodyParser(&body); err != nil {
			return fiberutil.BadRequest("Invalid request body")
		}
		if err := hostSvc.WriteTraefikDynamicFile(
			c.Context(),
			c.Params("serverId"),
			teamID,
			c.Params("filename"),
			body.Contents,
		); err != nil {
			return fiberutil.BadRequest(err.Error())
		}
		return fiberutil.OK(c, "Traefik file updated", map[string]any{
			"filename": c.Params("filename"),
		})
	})

	// Registry-credential CRUD — team-scoped saved docker registry
	// logins. Lives at the team root (not nested under a server)
	// since credentials are reusable across servers. Auth chain
	// drops the docker-specific RequireProvisionedServer middleware
	// because there's no serverId in the path.
	regCreds := router.Group("/registry-credentials",
		middleware.AuthenticatedChain(authMiddleware)...,
	)
	regCreds.Get("/", func(c *gofiber.Ctx) error {
		teamID, err := fiberutil.MustGetTeamID(c)
		if err != nil {
			return err
		}
		out, err := registryCredSvc.ListCredentials(c.Context(), teamID)
		if err != nil {
			return err
		}
		return fiberutil.OK(c, "Registry credentials retrieved", out)
	})
	regCreds.Post("/", func(c *gofiber.Ctx) error {
		teamID, userID, err := fiberutil.MustGetTeamAndUserID(c)
		if err != nil {
			return err
		}
		req, err := fiberutil.MustParseAndValidate[dto.CreateRegistryCredentialRequest](c)
		if err != nil {
			return err
		}
		out, err := registryCredSvc.CreateCredential(c.Context(), teamID, userID, req)
		if err != nil {
			return err
		}
		return fiberutil.Created(c, "Registry credential created", out)
	})
	regCreds.Patch("/:id", func(c *gofiber.Ctx) error {
		teamID, userID, err := fiberutil.MustGetTeamAndUserID(c)
		if err != nil {
			return err
		}
		req, err := fiberutil.MustParseAndValidate[dto.UpdateRegistryCredentialRequest](c)
		if err != nil {
			return err
		}
		out, err := registryCredSvc.UpdateCredential(c.Context(), c.Params("id"), teamID, userID, req)
		if err != nil {
			return err
		}
		return fiberutil.OK(c, "Registry credential updated", out)
	})
	regCreds.Delete("/:id", func(c *gofiber.Ctx) error {
		teamID, userID, err := fiberutil.MustGetTeamAndUserID(c)
		if err != nil {
			return err
		}
		if err := registryCredSvc.DeleteCredential(c.Context(), c.Params("id"), teamID, userID); err != nil {
			return err
		}
		return fiberutil.NoContent(c)
	})
}
