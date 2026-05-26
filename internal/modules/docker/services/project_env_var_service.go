package services

import (
	"context"
	"regexp"
	"strings"

	"github.com/kkz6/launch-go/internal/modules/docker/dto"
	"github.com/kkz6/launch-go/internal/modules/docker/models"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// ProjectEnvVarService manages project-scoped env vars. Mirrors the
// existing EnvVarService (application-scoped) — same key validation,
// secret-masking, bulk-set semantics. Mutations DON'T redeploy any
// dependent containers; the user clicks Redeploy on each that uses
// `${{project.<KEY>}}` to pick up the change.
type ProjectEnvVarService struct {
	*BaseService
}

func NewProjectEnvVarService(deps *ServiceDeps) *ProjectEnvVarService {
	return &ProjectEnvVarService{BaseService: NewBaseService(deps)}
}

// projectEnvKeyPattern is the legal-shape check for KEYs. Same
// alphabet POSIX shell allows: starts with letter/underscore, then
// alnum/underscore. Matches the interpolation regex's capture group
// so user-typed `${{project.X}}` keys can always be looked up.
var projectEnvKeyPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func (s *ProjectEnvVarService) requireProject(
	ctx context.Context, projectID, serverID, teamID string,
) error {
	_, err := s.Repos().Project().FindByIDAndTeamServer(ctx, projectID, teamID, serverID)
	return err
}

// ListEnvVars returns env vars for a project. Secrets are masked;
// pass reveal=true to GetEnvVar to read one in cleartext.
func (s *ProjectEnvVarService) ListEnvVars(
	ctx context.Context, projectID, serverID, teamID string,
) ([]dto.ProjectEnvVarResponse, error) {
	if err := s.requireProject(ctx, projectID, serverID, teamID); err != nil {
		return nil, err
	}
	rows, err := s.Repos().ProjectEnvVar().ListForProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	out := make([]dto.ProjectEnvVarResponse, 0, len(rows))
	for i := range rows {
		out = append(out, *dto.ToProjectEnvVarResponse(&rows[i], false))
	}
	return out, nil
}

// GetEnvVar returns one row with optional cleartext reveal — used by
// the "show value" eye-toggle in the UI.
func (s *ProjectEnvVarService) GetEnvVar(
	ctx context.Context, id, projectID, serverID, teamID string, reveal bool,
) (dto.ProjectEnvVarResponse, error) {
	if err := s.requireProject(ctx, projectID, serverID, teamID); err != nil {
		return dto.ProjectEnvVarResponse{}, err
	}
	v, err := s.Repos().ProjectEnvVar().FindByID(ctx, id)
	if err != nil {
		return dto.ProjectEnvVarResponse{}, err
	}
	if v.ProjectID != projectID {
		return dto.ProjectEnvVarResponse{}, fiberutil.NotFound()
	}
	return *dto.ToProjectEnvVarResponse(v, reveal), nil
}

// CreateEnvVar adds a key/value pair to the project. Key shape +
// uniqueness are validated server-side so a malformed reference
// can't sneak into a container's value.
func (s *ProjectEnvVarService) CreateEnvVar(
	ctx context.Context, projectID, serverID, teamID, userID string,
	req *dto.CreateEnvVarRequest,
) (dto.ProjectEnvVarResponse, error) {
	_ = userID
	if err := s.requireProject(ctx, projectID, serverID, teamID); err != nil {
		return dto.ProjectEnvVarResponse{}, err
	}
	key := strings.TrimSpace(req.Key)
	if !projectEnvKeyPattern.MatchString(key) {
		return dto.ProjectEnvVarResponse{}, fiberutil.BadRequest(
			"Key must match [A-Za-z_][A-Za-z0-9_]*",
		)
	}
	taken, err := s.Repos().ProjectEnvVar().ExistsByKey(ctx, projectID, key, "")
	if err != nil {
		return dto.ProjectEnvVarResponse{}, err
	}
	if taken {
		return dto.ProjectEnvVarResponse{}, fiberutil.Conflict(
			"An env var with that key already exists on this project",
		)
	}
	v := &models.ProjectEnvVar{
		ProjectID: projectID,
		Key:       key,
		Value:     dbtype.EncryptedString(req.Value),
		IsSecret:  req.IsSecret,
	}
	if err := s.Repos().ProjectEnvVar().Create(ctx, v); err != nil {
		return dto.ProjectEnvVarResponse{}, err
	}
	s.BroadcastToTeam(teamID, "docker.project.env_var.created", map[string]any{
		"project_id": projectID,
		"team_id":    teamID,
		"id":         v.ID,
		"key":        v.Key,
	})
	return *dto.ToProjectEnvVarResponse(v, false), nil
}

// UpdateEnvVar mutates value and/or is_secret. Key is immutable —
// removing + re-adding is the supported "rename" path so existing
// `${{project.<OLD>}}` references don't silently re-bind.
func (s *ProjectEnvVarService) UpdateEnvVar(
	ctx context.Context, id, projectID, serverID, teamID, userID string,
	req *dto.UpdateEnvVarRequest,
) (dto.ProjectEnvVarResponse, error) {
	_ = userID
	if err := s.requireProject(ctx, projectID, serverID, teamID); err != nil {
		return dto.ProjectEnvVarResponse{}, err
	}
	v, err := s.Repos().ProjectEnvVar().FindByID(ctx, id)
	if err != nil {
		return dto.ProjectEnvVarResponse{}, err
	}
	if v.ProjectID != projectID {
		return dto.ProjectEnvVarResponse{}, fiberutil.NotFound()
	}
	updates := map[string]any{}
	if req.Value != nil {
		updates["value"] = dbtype.EncryptedString(*req.Value)
	}
	if req.IsSecret != nil {
		updates["is_secret"] = *req.IsSecret
	}
	if len(updates) == 0 {
		return *dto.ToProjectEnvVarResponse(v, false), nil
	}
	if err := s.Repos().ProjectEnvVar().UpdateFields(ctx, v.ID, updates); err != nil {
		return dto.ProjectEnvVarResponse{}, err
	}
	reloaded, err := s.Repos().ProjectEnvVar().FindByID(ctx, v.ID)
	if err != nil {
		return dto.ProjectEnvVarResponse{}, err
	}
	s.BroadcastToTeam(teamID, "docker.project.env_var.updated", map[string]any{
		"project_id": projectID,
		"team_id":    teamID,
		"id":         reloaded.ID,
		"key":        reloaded.Key,
	})
	return *dto.ToProjectEnvVarResponse(reloaded, false), nil
}

// DeleteEnvVar soft-deletes a project env var. Containers that
// reference `${{project.<KEY>}}` keep working until they're next
// redeployed — at which point the resolver will leave the reference
// literal (no auto-rebind to anything else).
func (s *ProjectEnvVarService) DeleteEnvVar(
	ctx context.Context, id, projectID, serverID, teamID, userID string,
) error {
	_ = userID
	if err := s.requireProject(ctx, projectID, serverID, teamID); err != nil {
		return err
	}
	v, err := s.Repos().ProjectEnvVar().FindByID(ctx, id)
	if err != nil {
		return err
	}
	if v.ProjectID != projectID {
		return fiberutil.NotFound()
	}
	if err := s.Repos().ProjectEnvVar().Delete(ctx, v.ID); err != nil {
		return err
	}
	s.BroadcastToTeam(teamID, "docker.project.env_var.deleted", map[string]any{
		"project_id": projectID,
		"team_id":    teamID,
		"id":         v.ID,
		"key":        v.Key,
	})
	return nil
}
