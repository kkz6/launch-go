package services

import (
	"context"
	"strings"
	"time"

	"github.com/kkz6/launch-go/internal/modules/docker/dto"
	"github.com/kkz6/launch-go/internal/modules/docker/jobs"
	"github.com/kkz6/launch-go/internal/modules/docker/models"
	dockertypes "github.com/kkz6/launch-go/internal/modules/docker/types"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
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
	s.BroadcastToTeam(teamID, "docker.compose.created", resp)
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
	s.BroadcastToTeam(teamID, "docker.compose.updated", resp)
	return *resp, nil
}

// DeleteCompose soft-deletes the stack. Stopping the actual running
// containers via `docker compose down` lands in a follow-up — for now
// the abandoned containers will be reaped by the future cleanup job.
func (s *ComposeService) DeleteCompose(
	ctx context.Context, id, projectID, serverID, teamID, userID string,
) error {
	_ = userID
	if _, err := s.requireProjectScoped(ctx, projectID, serverID, teamID); err != nil {
		return err
	}
	c, err := s.Repos().Compose().FindByIDAndTeamServer(ctx, id, teamID, serverID)
	if err != nil {
		return err
	}
	if c.ProjectID != projectID {
		return fiberutil.NotFound()
	}
	if err := s.Repos().Compose().Delete(ctx, id); err != nil {
		return err
	}
	s.BroadcastToTeam(teamID, "docker.compose.deleted", map[string]any{
		"id":         c.ID,
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
