package services

import (
	"context"
	"strings"
	"time"

	"fmt"

	"github.com/kkz6/launch-go/internal/modules/docker/dto"
	"github.com/kkz6/launch-go/internal/modules/docker/jobs"
	"github.com/kkz6/launch-go/internal/modules/docker/models"
	"github.com/kkz6/launch-go/internal/modules/docker/tasks"
	dockertypes "github.com/kkz6/launch-go/internal/modules/docker/types"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// ComposeService owns CRUD + deploy for docker-compose stacks.
//
// Parallel in shape to ApplicationService — every method validates the
// project chain and emits a broadcast on success. The major behavioural
// difference is the deploy job dispatched in Deploy(), which runs
// `docker compose up -d` instead of `docker run`.
type ComposeService struct {
	*BaseService
}

// NewComposeService wires the service.
func NewComposeService(deps *ServiceDeps) *ComposeService {
	return &ComposeService{BaseService: NewBaseService(deps)}
}

// ListComposes returns every compose stack in a project. Raw YAML is
// stripped from list responses (it can be large) — only the Show
// endpoint includes it.
func (s *ComposeService) ListComposes(
	ctx context.Context, projectID, serverID, teamID string,
) ([]dto.ComposeResponse, error) {
	if _, err := s.requireProjectScoped(ctx, projectID, serverID, teamID); err != nil {
		return nil, err
	}
	rows, err := s.Repos().Compose().ListForProject(ctx, teamID, projectID)
	if err != nil {
		return nil, err
	}
	out := make([]dto.ComposeResponse, 0, len(rows))
	for i := range rows {
		out = append(out, *dto.ToComposeResponse(&rows[i], false))
	}
	return out, nil
}

// GetCompose returns one stack with the raw YAML included so the
// detail page can render it for editing/viewing.
func (s *ComposeService) GetCompose(
	ctx context.Context, id, projectID, serverID, teamID string,
) (dto.ComposeResponse, error) {
	if _, err := s.requireProjectScoped(ctx, projectID, serverID, teamID); err != nil {
		return dto.ComposeResponse{}, err
	}
	c, err := s.Repos().Compose().FindByIDAndTeamServer(ctx, id, teamID, serverID)
	if err != nil {
		return dto.ComposeResponse{}, err
	}
	if c.ProjectID != projectID {
		return dto.ComposeResponse{}, fiberutil.NotFound()
	}
	// Pull attached registry credentials so the detail page renders
	// chips without a second fetch. Best-effort — a load failure
	// just leaves the field empty.
	if err := s.DB().Model(c).
		Association("RegistryCredentials").
		Find(&c.RegistryCredentials); err != nil {
		s.LogError(err, "load compose registry credentials", "compose_id", id)
	}
	return *dto.ToComposeResponse(c, true), nil
}

// GetDefaultRunCommand renders the docker-suffix the deploy script
// falls back to when no per-stack RunCommand override is set. The
// Advanced subtab calls this to show the literal default in its
// "Default Command (...)" hint so the UI and the actual deploy can
// never drift — both go through tasks.ComposeDefaultRunCommand.
//
// Returns a plain string (not a DTO) — the route wraps it in a
// thin response object.
func (s *ComposeService) GetDefaultRunCommand(
	ctx context.Context, id, projectID, serverID, teamID string,
) (string, error) {
	project, err := s.Repos().Project().FindByIDAndTeamServer(ctx, projectID, teamID, serverID)
	if err != nil {
		return "", err
	}
	c, err := s.Repos().Compose().FindByIDAndTeamServer(ctx, id, teamID, serverID)
	if err != nil {
		return "", err
	}
	if c.ProjectID != projectID {
		return "", fiberutil.NotFound()
	}
	composeProjectName := fmt.Sprintf(
		"%s-%s",
		tasks.SlugFromName(project.Name),
		tasks.SlugFromName(c.Name),
	)
	// Compose file path depends on source: raw_yaml writes to
	// docker-compose.yml (the default), git uses the user-set path.
	composeFile := "docker-compose.yml"
	if c.ComposeSourceType == "git" && c.ComposeFilePath != nil && *c.ComposeFilePath != "" {
		composeFile = *c.ComposeFilePath
	}
	return tasks.ComposeDefaultRunCommand(composeProjectName, composeFile), nil
}

// CreateCompose registers a new compose stack. Source-type-specific
// payload is required and validated; the deploy step lands in
// Deploy() below.
func (s *ComposeService) CreateCompose(
	ctx context.Context, projectID, serverID, teamID, userID string,
	req *dto.CreateComposeRequest,
) (dto.ComposeResponse, error) {
	if _, err := s.requireProjectScoped(ctx, projectID, serverID, teamID); err != nil {
		return dto.ComposeResponse{}, err
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		return dto.ComposeResponse{}, fiberutil.BadRequest("Compose name is required")
	}
	taken, err := s.Repos().Compose().ExistsByNameInProject(ctx, projectID, name, "")
	if err != nil {
		return dto.ComposeResponse{}, err
	}
	if taken {
		return dto.ComposeResponse{}, fiberutil.Conflict(
			"A compose stack with that name already exists in this project",
		)
	}

	sourceConfig, composeFilePath, rawYAML, err := buildComposeSource(req)
	if err != nil {
		return dto.ComposeResponse{}, err
	}

	c := &models.Compose{
		ProjectID:         projectID,
		Name:              name,
		ComposeSourceType: req.ComposeSourceType,
		SourceConfig:      sourceConfig,
		ComposeFilePath:   composeFilePath,
		RawYAML:           rawYAML,
		Status:            dockertypes.ApplicationStatusIdle,
		BuildLocation:     dockertypes.BuildLocationServer,
	}
	c.TeamID = teamID
	c.ServerID = serverID
	// Stamp the creator (added in migration 0056). Used by the GHA
	// permissions-missing notification path. Nullable in the column
	// so missing userID stays silent rather than blowing up create.
	if userID != "" {
		uid := userID
		c.UserID = &uid
	}

	// Honour build_location on git-source composes. Only meaningful
	// for git source — the raw_yaml branch has no repo to commit a
	// workflow into; the validator on ComposeGitInput.BuildLocation
	// has already restricted the value space to {server, github_actions}.
	if req.ComposeSourceType == "git" && req.Git != nil && req.Git.BuildLocation != nil &&
		*req.Git.BuildLocation == string(dockertypes.BuildLocationGitHubActions) {
		c.BuildLocation = dockertypes.BuildLocationGitHubActions
	}

	if err := s.Repos().Compose().Create(ctx, c); err != nil {
		return dto.ComposeResponse{}, err
	}

	// Mirror the application-side bootstrap dispatch: if the compose
	// opted into GHA builds, kick off gha:bootstrap_workflow so the
	// workflow YAML + secret + variables land on the repo.
	if c.BuildLocation == dockertypes.BuildLocationGitHubActions {
		baseURL := s.AppURL()
		task, err := jobs.NewGHABootstrapWorkflowTask("compose", c.ID, true, baseURL)
		if err != nil {
			s.LogError(err, "build gha bootstrap task (compose)", "compose_id", c.ID)
		} else if err := s.EnqueueTask(task); err != nil {
			s.LogError(err, "enqueue gha bootstrap task (compose)", "compose_id", c.ID)
		}
	}

	// Attach saved registry credentials (many-to-many). Empty / nil
	// list = no auth on deploy. Each ID is verified to belong to the
	// caller's team before the join row lands — 404 for cross-team
	// picks (don't leak existence). Returned creds are loaded back
	// onto the model so the response carries the summary.
	if len(req.RegistryCredentialIDs) > 0 {
		creds, err := s.resolveRegistryCredentialsForTeam(ctx, req.RegistryCredentialIDs, teamID)
		if err != nil {
			return dto.ComposeResponse{}, err
		}
		if err := s.DB().Model(c).Association("RegistryCredentials").Replace(creds); err != nil {
			return dto.ComposeResponse{}, err
		}
		c.RegistryCredentials = creds
	}

	resp := dto.ToComposeResponse(c, false)
	s.BroadcastToTeam(teamID, "docker.compose.created", composeBroadcast(resp))
	return *resp, nil
}

// UpdateCompose renames a compose stack. Source/file changes need a
// reconfigure flow which lands in a follow-up.
func (s *ComposeService) UpdateCompose(
	ctx context.Context, id, projectID, serverID, teamID, userID string,
	req *dto.UpdateComposeRequest,
) (dto.ComposeResponse, error) {
	_ = userID
	if _, err := s.requireProjectScoped(ctx, projectID, serverID, teamID); err != nil {
		return dto.ComposeResponse{}, err
	}
	c, err := s.Repos().Compose().FindByIDAndTeamServer(ctx, id, teamID, serverID)
	if err != nil {
		return dto.ComposeResponse{}, err
	}
	if c.ProjectID != projectID {
		return dto.ComposeResponse{}, fiberutil.NotFound()
	}

	updates := map[string]any{}
	if req.Name != nil {
		newName := strings.TrimSpace(*req.Name)
		if newName == "" {
			return dto.ComposeResponse{}, fiberutil.BadRequest("Name cannot be empty")
		}
		if newName != c.Name {
			taken, err := s.Repos().Compose().ExistsByNameInProject(ctx, projectID, newName, id)
			if err != nil {
				return dto.ComposeResponse{}, err
			}
			if taken {
				return dto.ComposeResponse{}, fiberutil.Conflict(
					"A compose stack with that name already exists in this project",
				)
			}
			updates["name"] = newName
		}
	}
	// EnvFile is opt-in: nil leaves the column alone; an empty string
	// clears it; a non-empty string replaces it verbatim. The deploy
	// task writes this body to `${COMPOSE_DIR}/.env` before
	// `docker compose up`, so the change takes effect on the next
	// deploy without a restart job here.
	if req.EnvFile != nil {
		if *req.EnvFile == "" {
			updates["env_file"] = nil
		} else {
			updates["env_file"] = *req.EnvFile
		}
	}
	// RunCommand follows the same nil/empty/set semantics. Empty
	// string clears the override (deploy reverts to default);
	// non-empty replaces the docker suffix verbatim on next deploy.
	if req.RunCommand != nil {
		if strings.TrimSpace(*req.RunCommand) == "" {
			updates["run_command"] = nil
		} else {
			updates["run_command"] = *req.RunCommand
		}
	}
	if len(updates) > 0 {
		if err := s.Repos().Compose().UpdateFields(ctx, id, updates); err != nil {
			return dto.ComposeResponse{}, err
		}
	}

	// Replace the attached registry credentials in one shot. `nil` =
	// leave alone; an empty slice = detach all; non-empty = replace
	// with this exact set (de-duped + verified to belong to the team).
	// GORM's many2many Replace handles both insertion + deletion of
	// join rows.
	if req.RegistryCredentialIDs != nil {
		creds, err := s.resolveRegistryCredentialsForTeam(ctx, *req.RegistryCredentialIDs, teamID)
		if err != nil {
			return dto.ComposeResponse{}, err
		}
		if err := s.DB().Model(c).Association("RegistryCredentials").Replace(creds); err != nil {
			return dto.ComposeResponse{}, err
		}
	}

	reloaded, err := s.Repos().Compose().FindByIDAndTeamServer(ctx, id, teamID, serverID)
	if err != nil {
		return dto.ComposeResponse{}, err
	}
	// Preload the join so the response carries the summary chips.
	if err := s.DB().Model(reloaded).
		Association("RegistryCredentials").
		Find(&reloaded.RegistryCredentials); err != nil {
		s.LogError(err, "load compose registry credentials", "compose_id", id)
	}
	resp := dto.ToComposeResponse(reloaded, false)
	s.BroadcastToTeam(teamID, "docker.compose.updated", composeBroadcast(resp))
	return *resp, nil
}

// resolveRegistryCredentialsForTeam dedupes the incoming ID list +
// loads the matching rows scoped to the team. Returns 400 if any ID
// is missing from the team (cross-team picks are rejected the same
// way the application path handles a single credential).
//
// Returns an empty (non-nil) slice for an empty/nil input so the
// caller's `Association("…").Replace([])` semantics are predictable
// (Replace with empty = detach all).
func (s *ComposeService) resolveRegistryCredentialsForTeam(
	ctx context.Context, ids []string, teamID string,
) ([]models.RegistryCredential, error) {
	if len(ids) == 0 {
		return []models.RegistryCredential{}, nil
	}
	// Dedup while preserving order so the same picker submission
	// twice doesn't double up the join lookups.
	seen := make(map[string]struct{}, len(ids))
	uniqIDs := make([]string, 0, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		uniqIDs = append(uniqIDs, id)
	}
	creds, err := s.Repos().RegistryCredential().FindManyForTeam(ctx, uniqIDs, teamID)
	if err != nil {
		return nil, err
	}
	// Surface "ID not found" as a 400 so the user knows which input
	// was bad — silently dropping unknown IDs would let a stale UI
	// pick a credential that no longer exists in the team and look
	// like it succeeded.
	if len(creds) != len(uniqIDs) {
		return nil, fiberutil.BadRequest(
			"one or more registry_credential_ids do not belong to this team",
		)
	}
	return creds, nil
}

// DeleteCompose soft-deletes the stack AND dispatches a
// `docker compose down -v --remove-orphans` task so the containers
// the stack created go away on the host too. Audit 2026-05-23 flagged
// this — previously the row vanished from the UI while the containers
// kept running.
//
// Project name = `<project-slug>-<compose-slug>` — same value the
// deploy job uses for `--project-name`. We resolve it inline so the
// remove job doesn't need the (soft-deleted) row.
func (s *ComposeService) DeleteCompose(
	ctx context.Context, id, projectID, serverID, teamID, userID string,
	removeVolumes bool,
) error {
	_ = userID
	project, err := s.Repos().Project().FindByIDAndTeamServer(ctx, projectID, teamID, serverID)
	if err != nil {
		return err
	}
	c, err := s.Repos().Compose().FindByIDAndTeamServer(ctx, id, teamID, serverID)
	if err != nil {
		return err
	}
	if c.ProjectID != projectID {
		return fiberutil.NotFound()
	}

	// Resolve slugs unconditionally — the remove script uses them for
	// the stack-dir + traefik-config cleanup paths even when the
	// stack was never deployed (those paths might exist from a
	// previous deploy that was later rolled back).
	projectSlug := tasks.SlugFromName(project.Name)
	composeSlug := tasks.SlugFromName(c.Name)

	// composeProjectName drives the docker-label teardown (the
	// com.docker.compose.project label every container carries).
	// Skip when the stack was never deployed — no containers can
	// exist on the host yet, so the job's container-removal step
	// would just emit "no containers" noise.
	var composeProjectName string
	if c.LastDeployedAt != nil {
		composeProjectName = fmt.Sprintf("%s-%s", projectSlug, composeSlug)
	}

	if err := s.Repos().Compose().Delete(ctx, id); err != nil {
		return err
	}

	if rmTask, err := jobs.NewRemoveComposeTask(
		c.ID, c.ProjectID, c.ServerID, c.TeamID,
		composeProjectName, projectSlug, composeSlug,
		removeVolumes,
	); err == nil {
		if enqErr := s.EnqueueTask(rmTask); enqErr != nil {
			s.LogError(enqErr, "failed to dispatch compose removal", "compose_id", c.ID)
		}
	}

	s.BroadcastToTeam(teamID, "docker.compose.deleted", map[string]any{
		"id":         c.ID,
		"compose_id": c.ID,
		"project_id": c.ProjectID,
		"server_id":  c.ServerID,
		"team_id":    c.TeamID,
	})
	return nil
}

// PurgeComposeResources re-queues the label-based compose teardown
// for an already-deleted (or otherwise absent) stack. Normal use case:
// a compose was deleted while the old broken teardown script was in
// place, leaving containers running on the host. The DB row is gone,
// so the normal DeleteCompose path can't be used — this method
// accepts the project name (the com.docker.compose.project label
// value) and the optional path slugs and enqueues the same
// RemoveComposeJob that DeleteCompose would have dispatched.
//
// The server is looked up by ID + team so the endpoint can't be used
// across teams. A synthetic compose ID ("purge-<projectName>") is
// passed so the job's broadcast payload carries something identifiable
// in the server log, even though no DB row backs it.
func (s *ComposeService) PurgeComposeResources(
	ctx context.Context,
	serverID, teamID string,
	req *dto.PurgeComposeResourcesRequest,
) error {
	// FindByIDAndTeam already returns fiberutil.NotFound() for missing
	// rows and the original error otherwise — propagate as-is so a DB
	// connection blip surfaces as 500, not a misleading 404.
	if _, err := s.ServerRepos().Server().FindByIDAndTeam(ctx, serverID, teamID); err != nil {
		return err
	}

	syntheticID := fmt.Sprintf("purge-%s", req.ProjectName)
	rmTask, err := jobs.NewRemoveComposeTask(
		syntheticID, "", serverID, teamID,
		req.ProjectName,
		req.ProjectSlug,
		req.ComposeSlug,
		req.RemoveVolumes,
	)
	if err != nil {
		return fmt.Errorf("build purge task: %w", err)
	}
	if enqErr := s.EnqueueTask(rmTask); enqErr != nil {
		return fmt.Errorf("enqueue purge job: %w", enqErr)
	}

	s.BroadcastToTeam(teamID, "docker.compose.removed", map[string]any{
		"compose_id": syntheticID,
		"project_id": "",
		"server_id":  serverID,
		"team_id":    teamID,
	})
	return nil
}

// ListDeployments returns the deploy history rows for a compose stack.
// Uses the same polymorphic docker_deployments table — target_type=
// "compose" is the discriminator.
func (s *ComposeService) ListDeployments(
	ctx context.Context, composeID, projectID, serverID, teamID string,
) ([]models.Deployment, error) {
	if _, err := s.requireProjectScoped(ctx, projectID, serverID, teamID); err != nil {
		return nil, err
	}
	c, err := s.Repos().Compose().FindByIDAndTeamServer(ctx, composeID, teamID, serverID)
	if err != nil {
		return nil, err
	}
	if c.ProjectID != projectID {
		return nil, fiberutil.NotFound()
	}
	return s.Repos().Deployment().ListForTarget(ctx, "compose", composeID)
}

// DeleteDeployment removes a compose deployment history row, optionally deleting
// the GitHub Actions run too (best-effort-first: a GHA failure keeps the local
// row so the user can retry).
func (s *ComposeService) DeleteDeployment(
	ctx context.Context, composeID, projectID, serverID, teamID, deploymentID string, deleteFromGHA bool,
) error {
	if _, err := s.requireProjectScoped(ctx, projectID, serverID, teamID); err != nil {
		return err
	}
	c, err := s.Repos().Compose().FindByIDAndTeamServer(ctx, composeID, teamID, serverID)
	if err != nil {
		return err
	}
	if c.ProjectID != projectID {
		return fiberutil.NotFound()
	}
	dep, err := s.Repos().Deployment().FindByID(ctx, deploymentID)
	if err != nil {
		return err
	}
	if dep.TargetType != "compose" || dep.TargetID != composeID {
		return fiberutil.NotFound()
	}

	if deleteFromGHA && dep.GHARunID != nil && *dep.GHARunID != "" {
		if err := deleteGHAWorkflowRun(ctx, s.DB(), s.GitProviders(), c.SourceConfig, *dep.GHARunID); err != nil {
			return err
		}
	}

	return s.Repos().Deployment().Delete(ctx, deploymentID)
}

// GetDeploymentGHASteps returns the GitHub Actions step timeline for a
// compose deployment (#87). Same shape as the application path.
func (s *ComposeService) GetDeploymentGHASteps(
	ctx context.Context, composeID, projectID, serverID, teamID, deploymentID string,
) (*dto.DeploymentGHASteps, error) {
	if _, err := s.requireProjectScoped(ctx, projectID, serverID, teamID); err != nil {
		return nil, err
	}
	c, err := s.Repos().Compose().FindByIDAndTeamServer(ctx, composeID, teamID, serverID)
	if err != nil {
		return nil, err
	}
	if c.ProjectID != projectID {
		return nil, fiberutil.NotFound()
	}
	dep, err := s.Repos().Deployment().FindByID(ctx, deploymentID)
	if err != nil {
		return nil, err
	}
	if dep.TargetType != "compose" || dep.TargetID != composeID {
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

	wfJobs, err := fetchGHARunSteps(ctx, s.DB(), s.GitProviders(), c.SourceConfig, *dep.GHARunID)
	if err != nil {
		return nil, err
	}
	return buildGHAStepsResponse(dep.ID, *dep.GHARunID, runURL, wfJobs), nil
}

// Reload recreates the compose stack with the current .env and NO
// rebuild — `docker compose up -d --remove-orphans` reusing the images
// already on the host — so saved runtime env changes apply fast.
// Unlike Deploy it never routes GHA stacks to a workflow_dispatch: it
// re-ups the local images directly (build-time changes still need a
// full Deploy). It's a lightweight, toast-only action: NO deployment
// history row and NO build logs — the deploy job runs in recreate mode
// with an empty deployment ID, which makes it skip every deployment-row
// write and just broadcast the running/errored status when done.
func (s *ComposeService) Reload(
	ctx context.Context, composeID, projectID, serverID, teamID, userID string,
) error {
	_ = userID
	if _, err := s.requireProjectScoped(ctx, projectID, serverID, teamID); err != nil {
		return err
	}
	c, err := s.Repos().Compose().FindByIDAndTeamServer(ctx, composeID, teamID, serverID)
	if err != nil {
		return err
	}
	if c.ProjectID != projectID {
		return fiberutil.NotFound()
	}
	if c.Status == dockertypes.ApplicationStatusBuilding {
		return fiberutil.Conflict("A deployment is already in progress for this compose stack")
	}
	if c.LastDeployedAt == nil {
		return fiberutil.Conflict("Deploy this compose stack first — there's nothing to reload yet.")
	}

	task, err := jobs.NewRecreateComposeTask(composeID, serverID, teamID)
	if err != nil {
		return err
	}
	if err := s.EnqueueTask(task); err != nil {
		return err
	}

	s.BroadcastToTeam(teamID, "docker.compose.deploying", map[string]any{
		"compose_id": c.ID,
		"server_id":  serverID,
		"team_id":    teamID,
		"status":     "restarting",
	})
	return nil
}

// Deploy enqueues a compose deploy. Same shape as
// ApplicationService.Deploy — pending row + asynq job that does the
// actual SSH work.
func (s *ComposeService) Deploy(
	ctx context.Context, composeID, projectID, serverID, teamID, userID string,
) (*models.Deployment, error) {
	_ = userID
	if _, err := s.requireProjectScoped(ctx, projectID, serverID, teamID); err != nil {
		return nil, err
	}
	c, err := s.Repos().Compose().FindByIDAndTeamServer(ctx, composeID, teamID, serverID)
	if err != nil {
		return nil, err
	}
	if c.ProjectID != projectID {
		return nil, fiberutil.NotFound()
	}
	if c.Status == dockertypes.ApplicationStatusBuilding {
		return nil, fiberutil.Conflict("A deployment is already in progress for this compose stack")
	}

	// GitHub-Actions branch — same rationale as
	// ApplicationService.Deploy: when build_location=github_actions,
	// fire a workflow_dispatch on the customer's repo instead of
	// running the on-server compose build path. The eventual run
	// notifies us back via /api/webhooks/docker/composes/.../deploy.
	if c.BuildLocation == dockertypes.BuildLocationGitHubActions {
		return s.deployViaGitHubActions(ctx, c, serverID, teamID)
	}

	now := time.Now().UTC()
	deployment := &models.Deployment{
		TargetType: "compose",
		TargetID:   composeID,
		Status:     dockertypes.DeploymentStatusPending,
		StartedAt:  &now,
	}
	deployment.TeamID = teamID
	deployment.ServerID = serverID

	if err := s.Repos().Deployment().Create(ctx, deployment); err != nil {
		return nil, err
	}

	task, err := jobs.NewDeployComposeTask(composeID, deployment.ID, serverID, teamID)
	if err != nil {
		return nil, err
	}
	if err := s.EnqueueTask(task); err != nil {
		// Same fail-fast pattern as application deploy — mark the row
		// failed so we don't leak a pending row that will never run.
		errMsg := err.Error()
		finishedAt := time.Now().UTC()
		_ = s.Repos().Deployment().UpdateFields(ctx, deployment.ID, map[string]any{
			"status":      dockertypes.DeploymentStatusFailed,
			"finished_at": finishedAt,
			"error":       "failed to enqueue deploy job: " + errMsg,
		})
		return nil, err
	}

	s.BroadcastToTeam(teamID, "docker.compose.deploying", map[string]any{
		"compose_id":    c.ID,
		"deployment_id": deployment.ID,
		"server_id":     serverID,
		"team_id":       teamID,
		"status":        "pending",
	})
	return deployment, nil
}

// ListServices returns the docker compose service names currently
// known to the stack on the host. Drives the Logs subtab's service
// picker — the user sees one row per service and can scope the log
// stream to just that container's stdout.
//
// We use `docker compose --project-name <name> ps --services` rather
// than parsing the YAML because:
//   - It returns what's ACTUALLY on the host (handles partial
//     deploys, manually-removed containers, etc.) — the source of
//     truth for "which container could I tail right now".
//   - It works regardless of source type (raw_yaml vs git) without
//     us needing to parse YAML in Go.
//   - It's authoritative for compose-aware naming (sanitised dashes
//     and such).
//
// Empty list = stack has never been deployed (or all services were
// removed). The Logs subtab falls back to "all services" in that
// case.
func (s *ComposeService) ListServices(
	ctx context.Context, composeID, projectID, serverID, teamID string,
) ([]string, error) {
	if _, err := s.requireProjectScoped(ctx, projectID, serverID, teamID); err != nil {
		return nil, err
	}
	c, err := s.Repos().Compose().FindByIDAndTeamServer(ctx, composeID, teamID, serverID)
	if err != nil {
		return nil, err
	}
	if c.ProjectID != projectID {
		return nil, fiberutil.NotFound()
	}
	if c.LastDeployedAt == nil {
		// Never deployed → no services to list. Return [] rather
		// than 404 so the UI can render the dropdown empty.
		return []string{}, nil
	}

	project, err := s.Repos().Project().FindByIDAndTeamServer(ctx, projectID, teamID, serverID)
	if err != nil {
		return nil, err
	}
	projectName := fmt.Sprintf(
		"%s-%s",
		tasks.SlugFromName(project.Name),
		tasks.SlugFromName(c.Name),
	)

	server, err := s.ServerRepos().Server().FindByIDAndTeam(ctx, serverID, teamID)
	if err != nil {
		return nil, err
	}
	client, err := taskrunner.NewSSHClientFromServerAsRoot(server)
	if err != nil {
		return nil, err
	}
	if err := client.Connect(); err != nil {
		_ = client.Close()
		return nil, err
	}
	defer client.Close()

	// `--services` prints one service name per line. We don't need
	// the running-only filter; compose lists every service declared
	// in the compose file (`ps --services` doesn't require the
	// container to be running). That's intentional — the user might
	// want to tail a service that's currently stopped to see startup
	// failures.
	cmd := fmt.Sprintf(
		`docker compose --project-name %s ps --services 2>&1`,
		shellQuoteArg(projectName),
	)
	result, err := client.Run(ctx, cmd)
	if err != nil {
		return nil, err
	}
	if result.ExitCode != 0 {
		// Project label doesn't exist on host yet (deploy in flight
		// or the user manually `docker compose down`'d everything).
		// Surface as empty list rather than error so the UI gracefully
		// degrades.
		return []string{}, nil
	}

	var out []string
	for line := range strings.SplitSeq(strings.TrimSpace(result.Stdout), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			out = append(out, line)
		}
	}
	return out, nil
}

// shellQuoteArg single-quotes a shell argument; doubles up any
// embedded single quotes. Used for the compose --project-name value
// which we treat as untrusted user input even though SlugFromName
// already sanitises it (defence-in-depth).
func shellQuoteArg(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// requireProjectScoped is the compose-side equivalent of
// ApplicationService.requireProject. Mirrors that method intentionally
// so the two services have the same scoping behaviour.
func (s *ComposeService) requireProjectScoped(
	ctx context.Context, projectID, serverID, teamID string,
) (string, error) {
	p, err := s.Repos().Project().FindByIDAndTeamServer(ctx, projectID, teamID, serverID)
	if err != nil {
		return "", err
	}
	return p.ID, nil
}

// buildComposeSource validates the discriminated-union payload and
// returns the (source_config, compose_file_path, raw_yaml) triple the
// model stores.
func buildComposeSource(req *dto.CreateComposeRequest) (
	sourceConfig dbtype.JSONMap,
	composeFilePath *string,
	rawYAML *string,
	err error,
) {
	switch req.ComposeSourceType {
	case "git":
		if req.Git == nil ||
			strings.TrimSpace(req.Git.Repo) == "" ||
			strings.TrimSpace(req.Git.Branch) == "" {
			return nil, nil, nil, fiberutil.BadRequest("git source requires `git.repo` and `git.branch`")
		}
		sourceConfig = dbtype.JSONMap{
			"repo":   strings.TrimSpace(req.Git.Repo),
			"branch": strings.TrimSpace(req.Git.Branch),
		}
		if req.Git.SourceControlID != nil && *req.Git.SourceControlID != "" {
			sourceConfig["source_control_id"] = *req.Git.SourceControlID
		}
		if req.Git.ComposeFilePath != nil {
			trimmed := strings.TrimSpace(*req.Git.ComposeFilePath)
			if trimmed != "" {
				composeFilePath = &trimmed
			}
		}

	case "raw_yaml":
		if req.RawYAML == nil || strings.TrimSpace(req.RawYAML.Contents) == "" {
			return nil, nil, nil, fiberutil.BadRequest("raw_yaml source requires `raw_yaml.contents`")
		}
		// Store the YAML verbatim so redeploys reuse the exact text.
		contents := req.RawYAML.Contents
		rawYAML = &contents

	default:
		return nil, nil, nil, fiberutil.BadRequest("unsupported compose_source_type")
	}
	return sourceConfig, composeFilePath, rawYAML, nil
}

// composeBroadcast normalises a compose response for the WebSocket
// channel. The Vue listeners (e.g. compose/Deployments.vue) filter
// events by `data.compose_id`, but ComposeResponse's JSON key is
// `id`. Wrapping the response and inserting `compose_id` alongside
// keeps the API representation untouched while ensuring every
// docker.compose.* broadcast carries the routing field the client
// needs. Audit 2026-05-23 caught this — the renames broadcast was
// silently being dropped client-side.
func composeBroadcast(resp *dto.ComposeResponse) map[string]any {
	return map[string]any{
		"id":               resp.ID,
		"compose_id":       resp.ID,
		"team_id":          resp.TeamID,
		"server_id":        resp.ServerID,
		"project_id":       resp.ProjectID,
		"name":             resp.Name,
		"status":           resp.Status,
		"last_deployed_at": resp.LastDeployedAt,
	}
}
