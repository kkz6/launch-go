package services

import (
	"context"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/kkz6/launch-go/internal/modules/docker/dto"
	"github.com/kkz6/launch-go/internal/modules/docker/jobs"
	"github.com/kkz6/launch-go/internal/modules/docker/models"
	"github.com/kkz6/launch-go/internal/modules/docker/tasks"
	dockertypes "github.com/kkz6/launch-go/internal/modules/docker/types"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// ApplicationService handles business logic for docker applications.
type ApplicationService struct {
	*BaseService
}

// NewApplicationService wires the service from its dependency carrier.
func NewApplicationService(deps *ServiceDeps) *ApplicationService {
	return &ApplicationService{BaseService: NewBaseService(deps)}
}

// ListApplications returns every application inside a project, after
// verifying the project belongs to the (team, server) tuple. Returning
// 404 for a missing project (rather than 200 empty) means the frontend
// route can fail-fast on a bad project ID instead of silently rendering
// an empty list.
//
// Signature: fiberutil.IndexDoubleNestedFunc — (ctx, projectID,
// serverID, teamID) order.
func (s *ApplicationService) ListApplications(
	ctx context.Context, projectID, serverID, teamID string,
) ([]dto.ApplicationResponse, error) {
	if _, err := s.requireProject(ctx, projectID, serverID, teamID); err != nil {
		return nil, err
	}
	apps, err := s.Repos().Application().ListForProject(ctx, teamID, projectID)
	if err != nil {
		return nil, err
	}
	out := make([]dto.ApplicationResponse, 0, len(apps))
	for i := range apps {
		out = append(out, *dto.ToApplicationResponse(&apps[i]))
	}
	return out, nil
}

// GetApplication returns a single app by ID, after verifying it lives in
// the project the URL claims it does. Two scoping layers: the project
// lookup (404 if wrong team/server) AND the app's project_id (404 if the
// app exists but in a different project — a URL-tampering defence).
//
// Signature: fiberutil.ShowDoubleNestedFunc — (ctx, id, projectID,
// serverID, teamID).
func (s *ApplicationService) GetApplication(
	ctx context.Context, id, projectID, serverID, teamID string,
) (dto.ApplicationResponse, error) {
	// Load the project here (instead of just verifying it exists) so we
	// can stamp the deterministic container name onto the response —
	// the Terminal button on the application detail page reads it and
	// forwards as `?container=` to the WS handler, opening a shell
	// inside the container rather than on the host.
	project, err := s.Repos().Project().FindByIDAndTeamServer(ctx, projectID, teamID, serverID)
	if err != nil {
		return dto.ApplicationResponse{}, err
	}
	app, err := s.Repos().Application().FindByIDAndTeamServer(ctx, id, teamID, serverID)
	if err != nil {
		return dto.ApplicationResponse{}, err
	}
	if app.ProjectID != projectID {
		// App exists but belongs to a different project. Treat as NotFound
		// so an attacker can't enumerate apps by guessing URLs.
		return dto.ApplicationResponse{}, fiberutil.NotFound()
	}
	resp := dto.ToApplicationResponse(app)
	resp.ContainerName = tasks.ContainerNameFor(project, app)
	return *resp, nil
}

// CreateApplication registers a new docker application inside a project.
// Validates that:
//   - The project exists and belongs to the caller's team + server.
//   - The name is unique within the project.
//   - The source_type-specific payload is present and well-formed.
//
// The deploy job that actually pulls/builds/runs the container lands in
// slice 2b. For phase 2a, we just record the configuration with
// status=idle and broadcast docker.application.created.
//
// Signature: fiberutil.CreateDoubleNestedFunc — (ctx, projectID,
// serverID, teamID, userID, req).
func (s *ApplicationService) CreateApplication(
	ctx context.Context, projectID, serverID, teamID, userID string,
	req *dto.CreateApplicationRequest,
) (dto.ApplicationResponse, error) {
	if _, err := s.requireProject(ctx, projectID, serverID, teamID); err != nil {
		return dto.ApplicationResponse{}, err
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		return dto.ApplicationResponse{}, fiberutil.BadRequest("Application name is required")
	}

	taken, err := s.Repos().Application().ExistsByNameInProject(ctx, projectID, name, "")
	if err != nil {
		return dto.ApplicationResponse{}, err
	}
	if taken {
		return dto.ApplicationResponse{}, fiberutil.Conflict(
			"An application with that name already exists in this project",
		)
	}

	sourceConfig, buildType, buildConfig, err := buildSourceConfig(req)
	if err != nil {
		return dto.ApplicationResponse{}, err
	}

	port := 80
	if req.InternalPort != nil && *req.InternalPort > 0 {
		port = *req.InternalPort
	}

	app := &models.Application{
		ProjectID:    projectID,
		Name:         name,
		InternalPort: port,
		SourceType:   dockertypes.SourceType(req.SourceType),
		SourceConfig: sourceConfig,
		BuildType:    buildType,
		BuildConfig:  buildConfig,
		Status:       dockertypes.ApplicationStatusIdle,
		// Default to on-server build. The git-source branch flips this
		// to github_actions when the customer picks the GHA radio in
		// the create sheet.
		BuildLocation: dockertypes.BuildLocationServer,
	}
	app.TeamID = teamID
	app.ServerID = serverID
	// Stamp the creator. Nullable in the column (migration 0056), but
	// new rows always have a real userID from the authenticated caller.
	// Used by the GHA permissions-missing path to email the right user.
	if userID != "" {
		uid := userID
		app.UserID = &uid
	}

	// Honour the build_location toggle on git-source applications. We
	// don't allow GHA builds on image/dockerfile sources — those have
	// no repo to commit a workflow into. The DTO validator already
	// restricted the string to the two enum values.
	if req.SourceType == "git" && req.Git != nil && req.Git.BuildLocation != nil &&
		*req.Git.BuildLocation == string(dockertypes.BuildLocationGitHubActions) {
		app.BuildLocation = dockertypes.BuildLocationGitHubActions
	}

	// Wire registry authentication — only meaningful for image
	// sources. The DTO field validation already capped lengths; the
	// invariant we enforce here is "at most one of saved-credential
	// path vs inline path". Both empty = public image, no `docker
	// login` step at deploy time.
	if req.SourceType == "image" && req.Image != nil {
		if err := s.applyRegistryAuthToApp(ctx, app, teamID, req.Image); err != nil {
			return dto.ApplicationResponse{}, err
		}
	}

	if err := s.Repos().Application().Create(ctx, app); err != nil {
		return dto.ApplicationResponse{}, err
	}

	// If the app opted into GHA builds, kick off the bootstrap job to
	// commit the workflow file + provision repo secret + variables.
	// Best-effort — a dispatch error doesn't fail the create; the user
	// can re-sync from the GitHub Actions detail subtab in slice I.
	if app.BuildLocation == dockertypes.BuildLocationGitHubActions {
		baseURL := s.appURL()
		task, err := jobs.NewGHABootstrapWorkflowTask("application", app.ID, true, baseURL)
		if err != nil {
			s.LogError(err, "build gha bootstrap task", "application_id", app.ID)
		} else if err := s.EnqueueTask(task); err != nil {
			s.LogError(err, "enqueue gha bootstrap task", "application_id", app.ID)
		}
	}

	resp := dto.ToApplicationResponse(app)
	s.BroadcastToTeam(teamID, "docker.application.created", resp)
	return *resp, nil
}

// appURL returns the externally-reachable base URL the GHA workflow
// should POST notifies to. Pulled from runtime config; fallback empty
// (the workflow embeds it literally, so an empty here means the
// resulting workflow won't be usable — surfaces immediately if app
// boot didn't wire AppURL).
func (s *ApplicationService) appURL() string {
	return s.AppURL()
}

// UpdateApplication renames an application. Other fields are immutable in
// phase 2a (see UpdateApplicationRequest). If we widen this, the unique-
// name check needs to consider excludeID, which the repo already supports.
//
// Signature: fiberutil.UpdateDoubleNestedFunc.
func (s *ApplicationService) UpdateApplication(
	ctx context.Context, id, projectID, serverID, teamID, userID string,
	req *dto.UpdateApplicationRequest,
) (dto.ApplicationResponse, error) {
	_ = userID
	if _, err := s.requireProject(ctx, projectID, serverID, teamID); err != nil {
		return dto.ApplicationResponse{}, err
	}
	app, err := s.Repos().Application().FindByIDAndTeamServer(ctx, id, teamID, serverID)
	if err != nil {
		return dto.ApplicationResponse{}, err
	}
	if app.ProjectID != projectID {
		return dto.ApplicationResponse{}, fiberutil.NotFound()
	}

	updates := map[string]any{}
	if req.Name != nil {
		newName := strings.TrimSpace(*req.Name)
		if newName == "" {
			return dto.ApplicationResponse{}, fiberutil.BadRequest("Application name cannot be empty")
		}
		if newName != app.Name {
			taken, err := s.Repos().Application().ExistsByNameInProject(ctx, projectID, newName, id)
			if err != nil {
				return dto.ApplicationResponse{}, err
			}
			if taken {
				return dto.ApplicationResponse{}, fiberutil.Conflict(
					"An application with that name already exists in this project",
				)
			}
			updates["name"] = newName
		}
	}

	if len(updates) > 0 {
		if err := s.Repos().Application().UpdateFields(ctx, id, updates); err != nil {
			return dto.ApplicationResponse{}, err
		}
	}

	reloaded, err := s.Repos().Application().FindByIDAndTeamServer(ctx, id, teamID, serverID)
	if err != nil {
		return dto.ApplicationResponse{}, err
	}
	resp := dto.ToApplicationResponse(reloaded)
	s.BroadcastToTeam(teamID, "docker.application.updated", resp)
	return *resp, nil
}

// DeleteApplication soft-deletes the row AND dispatches a docker
// stop + docker rm task for the container the deploy job would have
// created. Audit 2026-05-23 flagged this — previously the row
// disappeared but the container kept running, leaking ports + name.
//
// The teardown runs as an asynq job (not inline) because docker stop
// can take 10+ seconds on apps with shutdown hooks, and the user
// expects the row to disappear instantly. Dispatch is best-effort:
// if Redis is unreachable we log and continue rather than failing
// the delete button.
//
// Signature: fiberutil.DeleteDoubleNestedFunc.
func (s *ApplicationService) DeleteApplication(
	ctx context.Context, id, projectID, serverID, teamID, userID string,
	removeVolumes bool,
) error {
	_ = userID
	project, err := s.requireProjectModel(ctx, projectID, serverID, teamID)
	if err != nil {
		return err
	}
	app, err := s.Repos().Application().FindByIDAndTeamServer(ctx, id, teamID, serverID)
	if err != nil {
		return err
	}
	if app.ProjectID != projectID {
		return fiberutil.NotFound()
	}

	// Resolve the container name BEFORE soft-deleting so the job
	// doesn't need an Unscoped query to recompute it. Apps that
	// never deployed have no container name; the job branches on
	// that and just broadcasts the no-op terminal event.
	var containerName string
	if app.LastDeployedAt != nil {
		containerName = tasks.ContainerNameFor(project, app)
	}

	// Resolve named volumes the app declared so the remove job can
	// `docker volume rm` each AFTER the container is gone. Bind +
	// file mounts don't need cleanup — bind paths belong to the
	// operator's host filesystem; file mounts are written under the
	// deploy dir which gets cleaned up separately. Only kind="volume"
	// (and the legacy "named" alias) entries map to real docker
	// volumes worth removing. Skipped entirely when removeVolumes is
	// false so a misclicked Delete preserves data.
	var volumeNames []string
	if removeVolumes {
		rows, listErr := s.Repos().Volume().ListForApplication(ctx, app.ID)
		if listErr != nil {
			// Don't block the delete on a volume-listing failure —
			// the container teardown is the higher priority. The
			// operator can `docker volume prune` later if needed.
			s.LogError(listErr, "failed to list app volumes before delete; volumes will be left in place", "application_id", app.ID)
		} else {
			for _, v := range rows {
				if v.Type != "volume" && v.Type != "named" {
					continue
				}
				if v.Name == "" {
					continue
				}
				volumeNames = append(volumeNames, v.Name)
			}
		}
	}

	// In-flight delete: flip status to "deleting" and broadcast
	// `docker.application.updated` so the UI shows a spinner + label
	// instead of the old "row vanishes the instant the API returns"
	// shape. The actual soft-delete + container teardown happens in
	// RemoveApplicationJob — on success it broadcasts
	// `docker.application.deleted` which removes the row; on failure
	// it reverts the status to `previousStatus` so the row stays
	// usable.
	//
	// Refuse a second Delete on an app already mid-delete (409) —
	// otherwise duplicate clicks would each dispatch a removal job
	// and the second one would race the soft-delete.
	if app.Status == dockertypes.ApplicationStatusDeleting {
		return fiberutil.Conflict("This application is already being deleted.")
	}
	previousStatus := string(app.Status)
	if err := s.Repos().Application().UpdateStatus(ctx, app.ID, string(dockertypes.ApplicationStatusDeleting)); err != nil {
		return err
	}

	s.BroadcastToTeam(teamID, "docker.application.updated", map[string]any{
		"id":         app.ID,
		"project_id": app.ProjectID,
		"server_id":  app.ServerID,
		"team_id":    app.TeamID,
		"status":     string(dockertypes.ApplicationStatusDeleting),
	})

	if rmTask, err := jobs.NewRemoveApplicationTask(
		app.ID, app.ProjectID, app.ServerID, app.TeamID, containerName, volumeNames, previousStatus,
	); err == nil {
		if enqErr := s.EnqueueTask(rmTask); enqErr != nil {
			// Couldn't queue — revert immediately so the row isn't
			// stuck on "deleting" forever waiting for a job that
			// never came.
			s.LogError(enqErr, "failed to dispatch app removal", "application_id", app.ID)
			_ = s.Repos().Application().UpdateStatus(ctx, app.ID, previousStatus)
			s.BroadcastToTeam(teamID, "docker.application.updated", map[string]any{
				"id":         app.ID,
				"project_id": app.ProjectID,
				"server_id":  app.ServerID,
				"team_id":    app.TeamID,
				"status":     previousStatus,
			})
			return fiberutil.Internal("Failed to queue the deletion. Try again.")
		}
	}
	return nil
}

// Lifecycle dispatches a single docker-side action (stop / restart /
// start) against the application's running container. Rebuild is NOT
// handled here — it's routed back through Deploy because rebuilding
// implies "fetch a fresh image / source and replace the container",
// which is exactly what Deploy already does. Reload in the UI maps
// to "restart" here.
//
// Returns 409 if the app has never deployed (nothing to act on).
func (s *ApplicationService) Lifecycle(
	ctx context.Context, applicationID, projectID, serverID, teamID, userID, action string,
) error {
	_ = userID
	project, err := s.requireProjectModel(ctx, projectID, serverID, teamID)
	if err != nil {
		return err
	}
	app, err := s.Repos().Application().FindByIDAndTeamServer(ctx, applicationID, teamID, serverID)
	if err != nil {
		return err
	}
	if app.ProjectID != projectID {
		return fiberutil.NotFound()
	}
	if app.LastDeployedAt == nil {
		return fiberutil.Conflict("Deploy this application first — there is no container to " + action + ".")
	}

	switch action {
	case "restart":
		// "Restart" is NOT a plain `docker restart` — that re-runs the
		// same container with its env baked in at create time, so
		// changed env vars would be ignored. Instead we RECREATE the
		// container from the image already on the host with the current
		// env/config (no rebuild) — which is what actually applies env
		// changes. It's a lightweight, toast-only action though: no
		// deployment history row and no build logs (the deploy job runs
		// in recreate mode with an empty deployment ID). See
		// recreateApplication.
		return s.recreateApplication(app)
	case "stop", "start":
		// ok — quick docker stop/start below.
	default:
		return fiberutil.BadRequest("Unsupported action: " + action)
	}

	containerName := tasks.ContainerNameFor(project, app)
	task, err := jobs.NewApplicationLifecycleTask(
		app.ID, app.ProjectID, app.ServerID, app.TeamID, containerName, action,
	)
	if err != nil {
		return err
	}
	if err := s.EnqueueTask(task); err != nil {
		return err
	}

	// Optimistic broadcast so the navbar pill flips immediately. The
	// worker will broadcast the terminal state when the docker call
	// returns; on failure it broadcasts status=errored.
	pending := map[string]string{
		"stop":  "stopping",
		"start": "starting",
	}[action]
	s.BroadcastToTeam(teamID, "docker.application.updated", map[string]any{
		"id":             app.ID,
		"application_id": app.ID,
		"project_id":     app.ProjectID,
		"server_id":      app.ServerID,
		"team_id":        app.TeamID,
		"status":         pending,
	})
	return nil
}

// recreateApplication implements "Restart" = recreate-with-current-env.
// It's a lightweight, toast-only action: it does NOT create a Deployment
// history row and does NOT stream build logs — the user just wants the
// container bounced with its current env applied. It enqueues the deploy
// job in recreate mode (reuse the on-host image, no build/pull) with an
// empty deployment ID, which makes the job skip every deployment-row
// write and just broadcast the running/errored status when done. Image
// presence + the env-file rewrite happen inside the job/script — see
// DeployApplicationPayload.Recreate.
func (s *ApplicationService) recreateApplication(app *models.Application) error {
	task, err := jobs.NewRecreateApplicationTask(app.ID, app.ServerID, app.TeamID)
	if err != nil {
		return err
	}
	if err := s.EnqueueTask(task); err != nil {
		return err
	}

	s.BroadcastToTeam(app.TeamID, "docker.application.updated", map[string]any{
		"id":             app.ID,
		"application_id": app.ID,
		"project_id":     app.ProjectID,
		"server_id":      app.ServerID,
		"team_id":        app.TeamID,
		"status":         "restarting",
	})
	return nil
}

// ListDeployments returns the deploy history for an application, most
// recent first. Cap is 50 rows (see DeploymentRepository.ListForTarget)
// so the response stays bounded even for hot apps with hundreds of
// redeploys over time.
//
// Signature: fiberutil.IndexDoubleNestedFunc — but we need the
// application ID as a third path param too. We accept (parentID,
// grandparentID, teamID) here as (projectID, serverID, teamID) and
// derive the appID from a different signature in the route closure.
// To keep things simple we expose a non-fiberutil signature and write
// the closure by hand in routes.go (see ListDeployments wrapper).
func (s *ApplicationService) ListDeployments(
	ctx context.Context, applicationID, projectID, serverID, teamID string,
) ([]models.Deployment, error) {
	if _, err := s.requireProject(ctx, projectID, serverID, teamID); err != nil {
		return nil, err
	}
	app, err := s.Repos().Application().FindByIDAndTeamServer(ctx, applicationID, teamID, serverID)
	if err != nil {
		return nil, err
	}
	if app.ProjectID != projectID {
		return nil, fiberutil.NotFound()
	}
	return s.Repos().Deployment().ListForTarget(ctx, "application", applicationID)
}

// DeleteDeployment removes a deployment history row. When deleteFromGHA is set
// and the deployment was run via GitHub Actions (carries a gha_run_id), the
// GitHub Actions run is deleted first; if that fails the local row is kept so
// the user can retry (or uncheck the option and delete just the local record).
func (s *ApplicationService) DeleteDeployment(
	ctx context.Context, applicationID, projectID, serverID, teamID, deploymentID string, deleteFromGHA bool,
) error {
	if _, err := s.requireProject(ctx, projectID, serverID, teamID); err != nil {
		return err
	}
	app, err := s.Repos().Application().FindByIDAndTeamServer(ctx, applicationID, teamID, serverID)
	if err != nil {
		return err
	}
	if app.ProjectID != projectID {
		return fiberutil.NotFound()
	}
	dep, err := s.Repos().Deployment().FindByID(ctx, deploymentID)
	if err != nil {
		return err
	}
	if dep.TargetType != "application" || dep.TargetID != applicationID {
		return fiberutil.NotFound()
	}

	if deleteFromGHA && dep.GHARunID != nil && *dep.GHARunID != "" {
		if err := deleteGHAWorkflowRun(ctx, s.DB(), s.GitProviders(), app.SourceConfig, *dep.GHARunID); err != nil {
			return err
		}
	}

	return s.Repos().Deployment().Delete(ctx, deploymentID)
}

// GetDeploymentGHASteps returns the GitHub Actions step timeline for a
// deployment (#87). Returns an empty, "pending" payload until the run id
// has been reported (the live deployment.gha_steps WS event covers that
// window); once known, it live-fetches the run's jobs/steps from GitHub.
func (s *ApplicationService) GetDeploymentGHASteps(
	ctx context.Context, applicationID, projectID, serverID, teamID, deploymentID string,
) (*dto.DeploymentGHASteps, error) {
	if _, err := s.requireProject(ctx, projectID, serverID, teamID); err != nil {
		return nil, err
	}
	app, err := s.Repos().Application().FindByIDAndTeamServer(ctx, applicationID, teamID, serverID)
	if err != nil {
		return nil, err
	}
	if app.ProjectID != projectID {
		return nil, fiberutil.NotFound()
	}
	dep, err := s.Repos().Deployment().FindByID(ctx, deploymentID)
	if err != nil {
		return nil, err
	}
	if dep.TargetType != "application" || dep.TargetID != applicationID {
		return nil, fiberutil.NotFound()
	}

	runURL := ""
	if dep.GHARunURL != nil {
		runURL = *dep.GHARunURL
	}
	if dep.GHARunID == nil || *dep.GHARunID == "" {
		return &dto.DeploymentGHASteps{
			DeploymentID: dep.ID,
			RunURL:       runURL,
			RunStatus:    "pending",
			Jobs:         []dto.DeploymentGHAJob{},
		}, nil
	}

	wfJobs, err := fetchGHARunSteps(ctx, s.DB(), s.GitProviders(), app.SourceConfig, *dep.GHARunID)
	if err != nil {
		return nil, err
	}
	return buildGHAStepsResponse(dep.ID, *dep.GHARunID, runURL, wfJobs), nil
}

// Deploy enqueues a deployment for the application. Creates a
// deployments row in `pending`, then asynchronously dispatches the
// docker:deploy_application asynq job which runs the SSH script.
//
// Returns the created deployment so the UI can render the new row
// immediately (status=pending) and the WS broadcasts catch up its
// lifecycle.
func (s *ApplicationService) Deploy(
	ctx context.Context, applicationID, projectID, serverID, teamID, userID string,
) (*models.Deployment, error) {
	_ = userID
	if _, err := s.requireProject(ctx, projectID, serverID, teamID); err != nil {
		return nil, err
	}
	app, err := s.Repos().Application().FindByIDAndTeamServer(ctx, applicationID, teamID, serverID)
	if err != nil {
		return nil, err
	}
	if app.ProjectID != projectID {
		return nil, fiberutil.NotFound()
	}
	// Guard against double-clicking Deploy while a previous run is in
	// flight. The deduplication key on the asynq task is the deployment
	// ID, but the user-facing 409 here gives a clearer error than asynq's
	// silent dedupe would.
	if app.Status == dockertypes.ApplicationStatusBuilding {
		return nil, fiberutil.Conflict("A deployment is already in progress for this application")
	}

	// A new trigger supersedes any prior in-flight rows for this workload
	// so a stale "Pending" placeholder (e.g. stranded by a cancelled GHA
	// run) flips to Cancelled instead of lingering forever (#103).
	// Best-effort — never block the deploy on it.
	if _, err := s.Repos().Deployment().SupersedeInProgressForTarget(ctx, "application", applicationID); err != nil {
		s.LogError(err, "supersede stale deployments", "application_id", applicationID)
	}

	// GitHub-Actions branch. When the application is configured to
	// build via GHA, the "Deploy" button MUST NOT enqueue the legacy
	// on-server docker:deploy_application job — that would clone the
	// repo and try to `docker build` on the target server, which is
	// the exact bug we're fixing. Instead, we fire a workflow_dispatch
	// on the customer's repo; GitHub Actions then builds + pushes the
	// image and webhooks Launch back via /api/webhooks/docker/...
	// which creates the real "deploying" deployment row.
	//
	// The pre-row we create here is a UX placeholder so the user sees
	// "Pending — GitHub Actions dispatched" immediately. The terminal
	// row will be a separate one keyed on the eventual gha_run_id —
	// acceptable for v1; we can dedupe later by claim-on-arrive if
	// the duplicate-row UX bothers anyone.
	if app.BuildLocation == dockertypes.BuildLocationGitHubActions {
		dep, err := s.deployViaGitHubActions(ctx, app, serverID, teamID)
		if err != nil {
			return nil, err
		}
		s.pruneDeploymentHistory(ctx, "application", applicationID)
		return dep, nil
	}

	now := time.Now().UTC()
	deployment := &models.Deployment{
		TargetType: "application",
		TargetID:   applicationID,
		Status:     dockertypes.DeploymentStatusPending,
		StartedAt:  &now,
	}
	deployment.TeamID = teamID
	deployment.ServerID = serverID

	if err := s.Repos().Deployment().Create(ctx, deployment); err != nil {
		return nil, err
	}

	task, err := jobs.NewDeployApplicationTask(applicationID, deployment.ID, serverID, teamID)
	if err != nil {
		return nil, err
	}
	if err := s.EnqueueTask(task); err != nil {
		// Don't leak a "pending" row that will never run — mark it
		// failed so the UI shows the dispatch error instead of a
		// permanent spinner.
		errMsg := err.Error()
		finishedAt := time.Now().UTC()
		_ = s.Repos().Deployment().UpdateFields(ctx, deployment.ID, map[string]any{
			"status":      dockertypes.DeploymentStatusFailed,
			"finished_at": finishedAt,
			"error":       "failed to enqueue deploy job: " + errMsg,
		})
		return nil, err
	}

	s.BroadcastToTeam(teamID, "docker.application.deploying", map[string]any{
		"application_id": app.ID,
		"deployment_id":  deployment.ID,
		"server_id":      serverID,
		"team_id":        teamID,
		"status":         "pending",
	})

	s.pruneDeploymentHistory(ctx, "application", applicationID)
	return deployment, nil
}

// pruneDeploymentHistory caps the application's retained deployment rows
// to deploymentHistoryKeep, deleting the oldest. Best-effort (#103).
func (s *ApplicationService) pruneDeploymentHistory(ctx context.Context, targetType, targetID string) {
	if err := s.Repos().Deployment().PruneForTarget(ctx, targetType, targetID, deploymentHistoryKeep); err != nil {
		s.LogError(err, "prune deployment history", "target_id", targetID)
	}
}

// UpdateAdvanced persists runtime knobs on the application by merging
// the request fields into build_config. We use build_config (not
// source_config) so source-only changes stay grouped logically and a
// future "reconfigure source" UI doesn't need to wade through CPU
// limits.
//
// Empty strings clear the value (delete from the JSON map); nil means
// "don't touch". Changes take effect on the next deploy.
func (s *ApplicationService) UpdateAdvanced(
	ctx context.Context, applicationID, projectID, serverID, teamID, userID string,
	req *dto.UpdateAdvancedRequest,
) (dto.ApplicationResponse, error) {
	_ = userID
	if _, err := s.requireProject(ctx, projectID, serverID, teamID); err != nil {
		return dto.ApplicationResponse{}, err
	}
	app, err := s.Repos().Application().FindByIDAndTeamServer(ctx, applicationID, teamID, serverID)
	if err != nil {
		return dto.ApplicationResponse{}, err
	}
	if app.ProjectID != projectID {
		return dto.ApplicationResponse{}, fiberutil.NotFound()
	}

	cfg := map[string]any(app.BuildConfig)
	if cfg == nil {
		cfg = map[string]any{}
	}
	applyAdvancedKey := func(key string, val *string) {
		if val == nil {
			return
		}
		if *val == "" {
			delete(cfg, key)
			return
		}
		cfg[key] = *val
	}
	applyAdvancedKey("cpu_limit", req.CPULimit)
	applyAdvancedKey("memory_limit", req.MemoryLimit)
	applyAdvancedKey("cpu_reservation", req.CPUReservation)
	applyAdvancedKey("memory_reservation", req.MemoryReservation)
	applyAdvancedKey("restart_policy", req.RestartPolicy)
	applyAdvancedKey("healthcheck_command", req.HealthcheckCommand)

	if req.ExtraPorts != nil {
		if len(req.ExtraPorts) == 0 {
			delete(cfg, "extra_ports")
		} else {
			cfg["extra_ports"] = req.ExtraPorts
		}
	}

	// Redirects: store as []map[string]any so the JSONMap round-trips
	// cleanly (validator already shaped the inputs). Empty slice =
	// clear the key. The next deploy emits Traefik RedirectRegex
	// middlewares from this slice.
	if req.Redirects != nil {
		if len(*req.Redirects) == 0 {
			delete(cfg, "redirects")
		} else {
			rows := make([]map[string]any, 0, len(*req.Redirects))
			for _, r := range *req.Redirects {
				rows = append(rows, map[string]any{
					"regex":       r.Regex,
					"replacement": r.Replacement,
					"permanent":   r.Permanent,
				})
			}
			cfg["redirects"] = rows
		}
	}

	// Security: nil = leave unchanged, empty username AND empty
	// password = clear the middleware (== no basic auth), otherwise
	// store both. Password is not encrypted at this layer — the row's
	// build_config is a plain JSON column. If we ship per-app secrets
	// at rest later this is the spot to swap to dbtype.EncryptedString.
	if req.Security != nil {
		if req.Security.Username == "" && req.Security.Password == "" {
			delete(cfg, "security")
		} else {
			cfg["security"] = map[string]any{
				"username": req.Security.Username,
				"password": req.Security.Password,
			}
		}
	}

	fields := map[string]any{
		"build_config": dbtype.JSONMap(cfg),
	}
	// InternalPort is a top-level column, not a build_config knob (#78).
	// Persist it when supplied so the next deploy regenerates the run/proxy
	// config on the new port.
	if req.InternalPort != nil && *req.InternalPort > 0 {
		fields["internal_port"] = *req.InternalPort
	}
	if err := s.Repos().Application().UpdateFields(ctx, app.ID, fields); err != nil {
		return dto.ApplicationResponse{}, err
	}

	reloaded, err := s.Repos().Application().FindByIDAndTeamServer(ctx, applicationID, teamID, serverID)
	if err != nil {
		return dto.ApplicationResponse{}, err
	}
	resp := dto.ToApplicationResponse(reloaded)
	s.BroadcastToTeam(teamID, "docker.application.updated", resp)
	return *resp, nil
}

// ─── Redirects (per-row CRUD over build_config.redirects) ─────────────
//
// Storage: build_config.redirects is a []map[string]any. Each row has
// an `id` (ULID generated server-side), `from`, `to`, `type`,
// `created_at`. The Application Redirects subtab on the frontend
// mirrors the PHP-site SitesRedirects subtab (same DataTable + add
// dialog) — these endpoints exist so that page can plug in cleanly.
//
// Why store in build_config (not a new table)? Redirects are a deploy
// concern — they compile to Traefik middlewares on the next deploy.
// Keeping them next to the other deploy knobs (resources, restart
// policy, healthcheck) means one fewer migration and one fewer join
// at deploy time.

// loadAndSaveRedirects is the shared read-modify-write helper. mutate
// receives the current slice (may be nil), returns the new slice. We
// run it inside a single UpdateFields write so concurrent edits race
// on Last-Write-Wins (acceptable here — the form serialises one row
// at a time).
func (s *ApplicationService) loadAndSaveRedirects(
	ctx context.Context, app *models.Application,
	mutate func([]map[string]any) ([]map[string]any, error),
) ([]map[string]any, error) {
	cfg := map[string]any(app.BuildConfig)
	if cfg == nil {
		cfg = map[string]any{}
	}
	var rows []map[string]any
	if raw, ok := cfg["redirects"].([]any); ok {
		for _, r := range raw {
			if m, ok := r.(map[string]any); ok {
				rows = append(rows, m)
			}
		}
	}
	next, err := mutate(rows)
	if err != nil {
		return nil, err
	}
	if len(next) == 0 {
		delete(cfg, "redirects")
	} else {
		cfg["redirects"] = next
	}
	if err := s.Repos().Application().UpdateFields(ctx, app.ID, map[string]any{
		"build_config": dbtype.JSONMap(cfg),
	}); err != nil {
		return nil, err
	}
	return next, nil
}

// requireApplication returns the application after checking it
// belongs to (team, server, project). Same defence-in-depth shape as
// requireProject — the redirect endpoints need the full model
// (not just the id) so we can mutate build_config in place.
func (s *ApplicationService) requireApplication(
	ctx context.Context, applicationID, projectID, serverID, teamID string,
) (*models.Application, error) {
	if _, err := s.requireProject(ctx, projectID, serverID, teamID); err != nil {
		return nil, err
	}
	app, err := s.Repos().Application().FindByIDAndTeamServer(ctx, applicationID, teamID, serverID)
	if err != nil {
		return nil, err
	}
	if app.ProjectID != projectID {
		return nil, fiberutil.NotFound()
	}
	return app, nil
}

func redirectRowToResponse(m map[string]any) dto.ApplicationRedirectResponse {
	r := dto.ApplicationRedirectResponse{}
	if v, ok := m["id"].(string); ok {
		r.ID = v
	}
	if v, ok := m["from"].(string); ok {
		r.From = v
	}
	if v, ok := m["to"].(string); ok {
		r.To = v
	}
	switch v := m["type"].(type) {
	case int:
		r.Type = v
	case float64:
		r.Type = int(v)
	}
	if v, ok := m["created_at"].(string); ok {
		r.CreatedAt = v
	}
	return r
}

// ListRedirects returns the per-app redirect rows in stored order
// (the user controls reordering via add/delete — no sort needed).
func (s *ApplicationService) ListRedirects(
	ctx context.Context, applicationID, projectID, serverID, teamID string,
) ([]dto.ApplicationRedirectResponse, error) {
	app, err := s.requireApplication(ctx, applicationID, projectID, serverID, teamID)
	if err != nil {
		return nil, err
	}
	cfg := map[string]any(app.BuildConfig)
	if cfg == nil {
		return []dto.ApplicationRedirectResponse{}, nil
	}
	raw, _ := cfg["redirects"].([]any)
	out := make([]dto.ApplicationRedirectResponse, 0, len(raw))
	for _, r := range raw {
		if m, ok := r.(map[string]any); ok {
			out = append(out, redirectRowToResponse(m))
		}
	}
	return out, nil
}

// CreateRedirect appends a row. ID is a fresh ULID. Returns the
// stored row so the UI can render it without a refetch.
func (s *ApplicationService) CreateRedirect(
	ctx context.Context, applicationID, projectID, serverID, teamID, userID string,
	req *dto.CreateApplicationRedirectRequest,
) (dto.ApplicationRedirectResponse, error) {
	_ = userID
	app, err := s.requireApplication(ctx, applicationID, projectID, serverID, teamID)
	if err != nil {
		return dto.ApplicationRedirectResponse{}, err
	}

	from := strings.TrimSpace(req.From)
	to := strings.TrimSpace(req.To)
	if from == "" || to == "" {
		return dto.ApplicationRedirectResponse{}, fiberutil.BadRequest("from and to are required")
	}

	row := map[string]any{
		"id":         ulid.Make().String(),
		"from":       from,
		"to":         to,
		"type":       req.Type,
		"created_at": time.Now().UTC().Format(time.RFC3339),
	}

	if _, err := s.loadAndSaveRedirects(ctx, app, func(rows []map[string]any) ([]map[string]any, error) {
		return append(rows, row), nil
	}); err != nil {
		return dto.ApplicationRedirectResponse{}, err
	}

	s.BroadcastToTeam(teamID, "docker.application.updated", map[string]any{
		"id":             app.ID,
		"application_id": app.ID,
		"project_id":     app.ProjectID,
		"server_id":      app.ServerID,
		"team_id":        app.TeamID,
	})
	return redirectRowToResponse(row), nil
}

// UpdateRedirect mutates a row by id. Returns 404 if no row matches.
func (s *ApplicationService) UpdateRedirect(
	ctx context.Context, applicationID, projectID, serverID, teamID, userID, redirectID string,
	req *dto.UpdateApplicationRedirectRequest,
) (dto.ApplicationRedirectResponse, error) {
	_ = userID
	app, err := s.requireApplication(ctx, applicationID, projectID, serverID, teamID)
	if err != nil {
		return dto.ApplicationRedirectResponse{}, err
	}

	var updated map[string]any
	if _, err := s.loadAndSaveRedirects(ctx, app, func(rows []map[string]any) ([]map[string]any, error) {
		for i, r := range rows {
			id, _ := r["id"].(string)
			if id != redirectID {
				continue
			}
			if req.From != nil {
				rows[i]["from"] = strings.TrimSpace(*req.From)
			}
			if req.To != nil {
				rows[i]["to"] = strings.TrimSpace(*req.To)
			}
			if req.Type != nil {
				rows[i]["type"] = *req.Type
			}
			updated = rows[i]
			return rows, nil
		}
		return rows, fiberutil.NotFound()
	}); err != nil {
		return dto.ApplicationRedirectResponse{}, err
	}

	s.BroadcastToTeam(teamID, "docker.application.updated", map[string]any{
		"id":             app.ID,
		"application_id": app.ID,
		"project_id":     app.ProjectID,
		"server_id":      app.ServerID,
		"team_id":        app.TeamID,
	})
	return redirectRowToResponse(updated), nil
}

// DeleteRedirect removes a row by id. 404 if no such row.
func (s *ApplicationService) DeleteRedirect(
	ctx context.Context, applicationID, projectID, serverID, teamID, userID, redirectID string,
) error {
	_ = userID
	app, err := s.requireApplication(ctx, applicationID, projectID, serverID, teamID)
	if err != nil {
		return err
	}
	found := false
	if _, err := s.loadAndSaveRedirects(ctx, app, func(rows []map[string]any) ([]map[string]any, error) {
		next := make([]map[string]any, 0, len(rows))
		for _, r := range rows {
			id, _ := r["id"].(string)
			if id == redirectID {
				found = true
				continue
			}
			next = append(next, r)
		}
		return next, nil
	}); err != nil {
		return err
	}
	if !found {
		return fiberutil.NotFound()
	}
	s.BroadcastToTeam(teamID, "docker.application.updated", map[string]any{
		"id":             app.ID,
		"application_id": app.ID,
		"project_id":     app.ProjectID,
		"server_id":      app.ServerID,
		"team_id":        app.TeamID,
	})
	return nil
}

// requireProject validates that the project exists and belongs to
// the (team, server). Returns the project ID on success — same shape
// as ProjectService.requireDockerServer so it slots into the same
// call pattern in each method.
func (s *ApplicationService) requireProject(
	ctx context.Context, projectID, serverID, teamID string,
) (string, error) {
	p, err := s.Repos().Project().FindByIDAndTeamServer(ctx, projectID, teamID, serverID)
	if err != nil {
		return "", err
	}
	return p.ID, nil
}

// requireProjectModel is the same check but returns the loaded
// model. Used where the caller needs the project Name to compute
// container names (delete teardown, etc.).
func (s *ApplicationService) requireProjectModel(
	ctx context.Context, projectID, serverID, teamID string,
) (*models.Project, error) {
	return s.Repos().Project().FindByIDAndTeamServer(ctx, projectID, teamID, serverID)
}

// buildSourceConfig translates the discriminated-union request payload
// into the (source_config jsonb, build_type, build_config jsonb) triple
// the model stores.
//
// Each branch verifies the matching nested struct is present so a client
// can't smuggle e.g. a Git payload while declaring source_type=image.
func buildSourceConfig(req *dto.CreateApplicationRequest) (
	source dbtype.JSONMap,
	buildType *dockertypes.BuildType,
	buildConfig dbtype.JSONMap,
	err error,
) {
	switch req.SourceType {
	case "image":
		if req.Image == nil || strings.TrimSpace(req.Image.Image) == "" {
			return nil, nil, nil, fiberutil.BadRequest("image source requires `image.image`")
		}
		source = dbtype.JSONMap{"image": strings.TrimSpace(req.Image.Image)}
		if req.Image.RegistryCredentialID != nil && *req.Image.RegistryCredentialID != "" {
			source["registry_credential_id"] = *req.Image.RegistryCredentialID
		}
		// No build step for pre-built images.

	case "git":
		if req.Git == nil ||
			strings.TrimSpace(req.Git.Repo) == "" ||
			strings.TrimSpace(req.Git.Branch) == "" {
			return nil, nil, nil, fiberutil.BadRequest("git source requires `git.repo` and `git.branch`")
		}
		source = dbtype.JSONMap{
			"repo":   strings.TrimSpace(req.Git.Repo),
			"branch": strings.TrimSpace(req.Git.Branch),
		}
		if req.Git.SourceControlID != nil && *req.Git.SourceControlID != "" {
			source["source_control_id"] = *req.Git.SourceControlID
		}
		// Build type defaults to nixpacks; the deploy job will auto-detect
		// a Dockerfile at the repo root and override unless the user has
		// explicitly chosen.
		bt := dockertypes.BuildTypeNixpacks
		if req.Git.BuildType != nil {
			bt = dockertypes.BuildType(*req.Git.BuildType)
		}
		buildType = &bt
		buildConfig = dbtype.JSONMap{}
		if req.Git.DockerfilePath != nil && strings.TrimSpace(*req.Git.DockerfilePath) != "" {
			buildConfig["dockerfile_path"] = strings.TrimSpace(*req.Git.DockerfilePath)
		}

	case "dockerfile":
		if req.Dockerfile == nil || strings.TrimSpace(req.Dockerfile.Contents) == "" {
			return nil, nil, nil, fiberutil.BadRequest("dockerfile source requires `dockerfile.contents`")
		}
		// We store the contents directly in source_config so a redeploy
		// uses the same Dockerfile without re-fetching. The 64 KiB cap on
		// the request struct keeps this bounded.
		source = dbtype.JSONMap{"contents": req.Dockerfile.Contents}
		bt := dockertypes.BuildTypeDockerfile
		buildType = &bt
		buildConfig = dbtype.JSONMap{}

	default:
		return nil, nil, nil, fiberutil.BadRequest("unsupported source_type")
	}
	return source, buildType, buildConfig, nil
}

// applyRegistryAuthToApp resolves the discriminated-union of registry-
// auth options on the image input + applies the result to the
// application model. Branches:
//
//   - Nothing set                 → public image, no auth on deploy.
//   - registry_credential_id only → reference a saved credential.
//     Verifies the credential belongs to
//     the caller's team (cross-team picks
//     are rejected as 404 — we don't leak
//     the existence of other teams' creds).
//   - username + password set     → inline credentials, encrypted at
//     rest via dbtype.EncryptedString.
//     registry_url is optional (empty →
//     Docker Hub at deploy time).
//   - mixed                       → rejected as 400.
//
// Caller is the create flow today. The update flow doesn't expose
// these (UpdateApplicationRequest is name-only); rotating credentials
// happens by editing the saved-credential row or deleting + recreating
// the application.
func (s *ApplicationService) applyRegistryAuthToApp(
	ctx context.Context, app *models.Application, teamID string,
	in *dto.ImageSourceInput,
) error {
	hasSaved := in.RegistryCredentialID != nil && *in.RegistryCredentialID != ""
	hasInline := (in.RegistryUsername != nil && *in.RegistryUsername != "") ||
		(in.RegistryPassword != nil && *in.RegistryPassword != "")

	if hasSaved && hasInline {
		return fiberutil.BadRequest(
			"choose either a saved registry credential OR inline credentials, not both",
		)
	}

	if hasSaved {
		// Team-scoped lookup — rejects cross-team picks.
		if _, err := s.Repos().RegistryCredential().FindByIDForTeam(
			ctx, *in.RegistryCredentialID, teamID,
		); err != nil {
			return err
		}
		app.RegistryCredentialID = in.RegistryCredentialID
		return nil
	}

	if hasInline {
		// Both username + password required for inline. Surfacing a
		// 400 here is friendlier than a half-set row that silently
		// fails on `docker login` at deploy time.
		if in.RegistryUsername == nil || *in.RegistryUsername == "" ||
			in.RegistryPassword == nil || *in.RegistryPassword == "" {
			return fiberutil.BadRequest(
				"inline registry auth requires both username AND password",
			)
		}
		username := strings.TrimSpace(*in.RegistryUsername)
		app.RegistryUsername = &username
		app.RegistryPassword = dbtype.EncryptedString(*in.RegistryPassword)
		// registry_url rides in source_config for the inline path —
		// it doesn't justify a column of its own (only meaningful
		// when inline auth is set). Saved credentials carry their
		// URL on the row instead.
		if in.RegistryURL != nil {
			url := strings.TrimSpace(*in.RegistryURL)
			if url != "" {
				if app.SourceConfig == nil {
					app.SourceConfig = dbtype.JSONMap{}
				}
				app.SourceConfig["registry_url"] = url
			}
		}
	}
	return nil
}
