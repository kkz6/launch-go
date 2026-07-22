package services

import (
	"context"
	"strings"

	"github.com/kkz6/launch-go/internal/modules/docker/dto"
	"github.com/kkz6/launch-go/internal/modules/docker/jobs"
	"github.com/kkz6/launch-go/internal/modules/docker/models"
	dockertypes "github.com/kkz6/launch-go/internal/modules/docker/types"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// ComposeBuildSecretService is the compose-stack mirror of
// BuildSecretService. Same auth/scoping/encryption/auto-resync rules,
// just keyed by compose_id.
type ComposeBuildSecretService struct {
	*BaseService
}

func NewComposeBuildSecretService(deps *ServiceDeps) *ComposeBuildSecretService {
	return &ComposeBuildSecretService{BaseService: NewBaseService(deps)}
}

func (s *ComposeBuildSecretService) ListBuildSecrets(
	ctx context.Context, composeID, projectID, serverID, teamID string,
) ([]dto.BuildSecretResponse, error) {
	if _, err := s.scopedCompose(ctx, composeID, projectID, serverID, teamID); err != nil {
		return nil, err
	}
	rows, err := s.Repos().ComposeBuildSecret().ListForCompose(ctx, composeID)
	if err != nil {
		return nil, err
	}
	return mapResponseValues(rows, dto.ToComposeBuildSecretResponse), nil
}

func (s *ComposeBuildSecretService) CreateBuildSecret(
	ctx context.Context, composeID, projectID, serverID, teamID, userID string,
	req *dto.CreateBuildSecretRequest,
) (dto.BuildSecretResponse, error) {
	_ = userID
	compose, err := s.scopedCompose(ctx, composeID, projectID, serverID, teamID)
	if err != nil {
		return dto.BuildSecretResponse{}, err
	}

	name := strings.TrimSpace(req.Name)
	if !validBuildSecretName.MatchString(name) {
		return dto.BuildSecretResponse{}, fiberutil.BadRequest(
			"Build secret name must start with a letter or underscore and contain only letters, digits, or underscores",
		)
	}
	taken, err := s.Repos().ComposeBuildSecret().ExistsByName(ctx, composeID, name, "")
	if err != nil {
		return dto.BuildSecretResponse{}, err
	}
	if taken {
		return dto.BuildSecretResponse{}, fiberutil.Conflict(
			"A build secret with that name already exists on this stack",
		)
	}

	v := &models.ComposeBuildSecret{
		ComposeID: composeID,
		Name:      name,
		Value:     dbtype.EncryptedString(req.Value),
	}
	if err := s.Repos().ComposeBuildSecret().Create(ctx, v); err != nil {
		return dto.BuildSecretResponse{}, err
	}

	resp := dto.ToComposeBuildSecretResponse(v)
	s.BroadcastToTeam(teamID, "docker.compose.build_secret.added", map[string]any{
		"compose_id":      compose.ID,
		"server_id":       compose.ServerID,
		"team_id":         compose.TeamID,
		"build_secret_id": v.ID,
		"name":            v.Name,
	})
	s.queueResyncIfGHA(compose)
	return *resp, nil
}

func (s *ComposeBuildSecretService) UpdateBuildSecret(
	ctx context.Context, id, composeID, projectID, serverID, teamID, userID string,
	req *dto.UpdateBuildSecretRequest,
) (dto.BuildSecretResponse, error) {
	_ = userID
	compose, err := s.scopedCompose(ctx, composeID, projectID, serverID, teamID)
	if err != nil {
		return dto.BuildSecretResponse{}, err
	}
	v, err := s.Repos().ComposeBuildSecret().FindByID(ctx, id)
	if err != nil {
		return dto.BuildSecretResponse{}, err
	}
	if v.ComposeID != composeID {
		return dto.BuildSecretResponse{}, fiberutil.NotFound()
	}
	if err := s.Repos().ComposeBuildSecret().UpdateFields(ctx, id, map[string]any{
		"value": dbtype.EncryptedString(req.Value),
	}); err != nil {
		return dto.BuildSecretResponse{}, err
	}
	reloaded, err := s.Repos().ComposeBuildSecret().FindByID(ctx, id)
	if err != nil {
		return dto.BuildSecretResponse{}, err
	}
	s.BroadcastToTeam(teamID, "docker.compose.build_secret.updated", map[string]any{
		"compose_id":      compose.ID,
		"server_id":       compose.ServerID,
		"team_id":         compose.TeamID,
		"build_secret_id": id,
		"name":            reloaded.Name,
	})
	s.queueResyncIfGHA(compose)
	return *dto.ToComposeBuildSecretResponse(reloaded), nil
}

func (s *ComposeBuildSecretService) DeleteBuildSecret(
	ctx context.Context, id, composeID, projectID, serverID, teamID, userID string,
) error {
	_ = userID
	compose, err := s.scopedCompose(ctx, composeID, projectID, serverID, teamID)
	if err != nil {
		return err
	}
	v, err := s.Repos().ComposeBuildSecret().FindByID(ctx, id)
	if err != nil {
		return err
	}
	if v.ComposeID != composeID {
		return fiberutil.NotFound()
	}
	name := v.Name
	if err := s.Repos().ComposeBuildSecret().Delete(ctx, id); err != nil {
		return err
	}
	s.BroadcastToTeam(teamID, "docker.compose.build_secret.deleted", map[string]any{
		"compose_id":      compose.ID,
		"server_id":       compose.ServerID,
		"team_id":         compose.TeamID,
		"build_secret_id": id,
		"name":            name,
	})
	s.queueResyncIfGHA(compose)
	return nil
}

func (s *ComposeBuildSecretService) scopedCompose(
	ctx context.Context, composeID, projectID, serverID, teamID string,
) (*models.Compose, error) {
	if _, err := s.Repos().Project().FindByIDAndTeamServer(ctx, projectID, teamID, serverID); err != nil {
		return nil, err
	}
	compose, err := s.Repos().Compose().FindByIDAndTeamServer(ctx, composeID, teamID, serverID)
	if err != nil {
		return nil, err
	}
	if compose.ProjectID != projectID {
		return nil, fiberutil.NotFound()
	}
	return compose, nil
}

// queueResyncIfGHA mirrors the application service helper. Re-syncs
// the compose workflow file + repo secrets when the stack is GHA-
// backed. Best-effort; failures are logged but don't fail the
// secret-save API call.
func (s *ComposeBuildSecretService) queueResyncIfGHA(compose *models.Compose) {
	if compose == nil || compose.BuildLocation != dockertypes.BuildLocationGitHubActions {
		return
	}
	task, err := jobs.NewGHABootstrapWorkflowTask("compose", compose.ID, false, s.AppURL())
	if err != nil {
		if s.Logger != nil {
			s.Logger.Warn().Err(err).Str("compose_id", compose.ID).
				Msg("build-secret: failed to build resync task (compose)")
		}
		return
	}
	if err := s.EnqueueTask(task); err != nil && s.Logger != nil {
		s.Logger.Warn().Err(err).Str("compose_id", compose.ID).
			Msg("build-secret: failed to queue workflow resync after secret change (compose)")
	}
}
