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
	return *dto.ToComposeResponse(c, true), nil
}

// CreateCompose registers a new compose stack. Source-type-specific
// payload is required and validated; the deploy step lands in
// Deploy() below.
func (s *ComposeService) CreateCompose(
	ctx context.Context, projectID, serverID, teamID, userID string,
	req *dto.CreateComposeRequest,
) (dto.ComposeResponse, error) {
	_ = userID
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
	}
	c.TeamID = teamID
	c.ServerID = serverID

	if err := s.Repos().Compose().Create(ctx, c); err != nil {
		return dto.ComposeResponse{}, err
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
	if len(updates) > 0 {
		if err := s.Repos().Compose().UpdateFields(ctx, id, updates); err != nil {
			return dto.ComposeResponse{}, err
		}
	}

	reloaded, err := s.Repos().Compose().FindByIDAndTeamServer(ctx, id, teamID, serverID)
	if err != nil {
		return dto.ComposeResponse{}, err
	}
	resp := dto.ToComposeResponse(reloaded, false)
	s.BroadcastToTeam(teamID, "docker.compose.updated", composeBroadcast(resp))
	return *resp, nil
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

	// Only compute the project name if the stack actually deployed.
	// Otherwise the down task is a no-op the job will short-circuit.
	var composeProjectName string
	if c.LastDeployedAt != nil {
		composeProjectName = fmt.Sprintf(
			"%s-%s",
			tasks.SlugFromName(project.Name),
			tasks.SlugFromName(c.Name),
		)
	}

	if err := s.Repos().Compose().Delete(ctx, id); err != nil {
		return err
	}

	if rmTask, err := jobs.NewRemoveComposeTask(
		c.ID, c.ProjectID, c.ServerID, c.TeamID, composeProjectName, removeVolumes,
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
	for _, line := range strings.Split(strings.TrimSpace(result.Stdout), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		out = append(out, line)
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
