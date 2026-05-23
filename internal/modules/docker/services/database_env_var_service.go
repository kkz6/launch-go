package services

import (
	"context"
	"strings"

	"github.com/kkz6/launch-go/internal/modules/docker/dto"
	"github.com/kkz6/launch-go/internal/modules/docker/models"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// DatabaseEnvVarService manages env vars on a managed database
// container. Same shape as EnvVarService (applications) and
// ProjectEnvVarService — kept separate because each carries a
// different FK column.
//
// Mutations don't restart the container. Next "Restart" or "Rebuild"
// pulls the fresh list and reapplies via `docker run -e` (engine
// creds + user env vars + project-ref resolution).
type DatabaseEnvVarService struct {
	*BaseService
}

func NewDatabaseEnvVarService(deps *ServiceDeps) *DatabaseEnvVarService {
	return &DatabaseEnvVarService{BaseService: NewBaseService(deps)}
}

func (s *DatabaseEnvVarService) requireDatabase(
	ctx context.Context, databaseID, projectID, serverID, teamID string,
) (*models.Database, error) {
	if _, err := s.Repos().Project().FindByIDAndTeamServer(ctx, projectID, teamID, serverID); err != nil {
		return nil, err
	}
	db, err := s.Repos().Database().FindByIDAndTeamServer(ctx, databaseID, teamID, serverID)
	if err != nil {
		return nil, err
	}
	if db.ProjectID != projectID {
		return nil, fiberutil.NotFound()
	}
	return db, nil
}

func (s *DatabaseEnvVarService) ListEnvVars(
	ctx context.Context, databaseID, projectID, serverID, teamID string,
) ([]dto.DatabaseEnvVarResponse, error) {
	if _, err := s.requireDatabase(ctx, databaseID, projectID, serverID, teamID); err != nil {
		return nil, err
	}
	rows, err := s.Repos().DatabaseEnvVar().ListForDatabase(ctx, databaseID)
	if err != nil {
		return nil, err
	}
	out := make([]dto.DatabaseEnvVarResponse, 0, len(rows))
	for i := range rows {
		out = append(out, *dto.ToDatabaseEnvVarResponse(&rows[i], false))
	}
	return out, nil
}

func (s *DatabaseEnvVarService) GetEnvVar(
	ctx context.Context, id, databaseID, projectID, serverID, teamID string, reveal bool,
) (dto.DatabaseEnvVarResponse, error) {
	if _, err := s.requireDatabase(ctx, databaseID, projectID, serverID, teamID); err != nil {
		return dto.DatabaseEnvVarResponse{}, err
	}
	v, err := s.Repos().DatabaseEnvVar().FindByID(ctx, id)
	if err != nil {
		return dto.DatabaseEnvVarResponse{}, err
	}
	if v.DatabaseID != databaseID {
		return dto.DatabaseEnvVarResponse{}, fiberutil.NotFound()
	}
	return *dto.ToDatabaseEnvVarResponse(v, reveal), nil
}

func (s *DatabaseEnvVarService) CreateEnvVar(
	ctx context.Context, databaseID, projectID, serverID, teamID, userID string,
	req *dto.CreateEnvVarRequest,
) (dto.DatabaseEnvVarResponse, error) {
	_ = userID
	if _, err := s.requireDatabase(ctx, databaseID, projectID, serverID, teamID); err != nil {
		return dto.DatabaseEnvVarResponse{}, err
	}
	key := strings.TrimSpace(req.Key)
	// Reuse the same key-shape regex used by the project env vars —
	// the run-script eventually feeds these through docker -e, which
	// requires shell-safe identifiers.
	if !projectEnvKeyPattern.MatchString(key) {
		return dto.DatabaseEnvVarResponse{}, fiberutil.BadRequest(
			"Key must match [A-Za-z_][A-Za-z0-9_]*",
		)
	}
	taken, err := s.Repos().DatabaseEnvVar().ExistsByKey(ctx, databaseID, key, "")
	if err != nil {
		return dto.DatabaseEnvVarResponse{}, err
	}
	if taken {
		return dto.DatabaseEnvVarResponse{}, fiberutil.Conflict(
			"An env var with that key already exists on this database",
		)
	}
	v := &models.DatabaseEnvVar{
		DatabaseID: databaseID,
		Key:        key,
		Value:      dbtype.EncryptedString(req.Value),
		IsSecret:   req.IsSecret,
	}
	if err := s.Repos().DatabaseEnvVar().Create(ctx, v); err != nil {
		return dto.DatabaseEnvVarResponse{}, err
	}
	s.BroadcastToTeam(teamID, "docker.database.env_var.created", map[string]any{
		"database_id": databaseID,
		"project_id":  projectID,
		"team_id":     teamID,
		"id":          v.ID,
		"key":         v.Key,
	})
	return *dto.ToDatabaseEnvVarResponse(v, false), nil
}

func (s *DatabaseEnvVarService) UpdateEnvVar(
	ctx context.Context, id, databaseID, projectID, serverID, teamID, userID string,
	req *dto.UpdateEnvVarRequest,
) (dto.DatabaseEnvVarResponse, error) {
	_ = userID
	if _, err := s.requireDatabase(ctx, databaseID, projectID, serverID, teamID); err != nil {
		return dto.DatabaseEnvVarResponse{}, err
	}
	v, err := s.Repos().DatabaseEnvVar().FindByID(ctx, id)
	if err != nil {
		return dto.DatabaseEnvVarResponse{}, err
	}
	if v.DatabaseID != databaseID {
		return dto.DatabaseEnvVarResponse{}, fiberutil.NotFound()
	}
	updates := map[string]any{}
	if req.Value != nil {
		updates["value"] = dbtype.EncryptedString(*req.Value)
	}
	if req.IsSecret != nil {
		updates["is_secret"] = *req.IsSecret
	}
	if len(updates) == 0 {
		return *dto.ToDatabaseEnvVarResponse(v, false), nil
	}
	if err := s.Repos().DatabaseEnvVar().UpdateFields(ctx, v.ID, updates); err != nil {
		return dto.DatabaseEnvVarResponse{}, err
	}
	reloaded, err := s.Repos().DatabaseEnvVar().FindByID(ctx, v.ID)
	if err != nil {
		return dto.DatabaseEnvVarResponse{}, err
	}
	s.BroadcastToTeam(teamID, "docker.database.env_var.updated", map[string]any{
		"database_id": databaseID,
		"project_id":  projectID,
		"team_id":     teamID,
		"id":          reloaded.ID,
		"key":         reloaded.Key,
	})
	return *dto.ToDatabaseEnvVarResponse(reloaded, false), nil
}

func (s *DatabaseEnvVarService) DeleteEnvVar(
	ctx context.Context, id, databaseID, projectID, serverID, teamID, userID string,
) error {
	_ = userID
	if _, err := s.requireDatabase(ctx, databaseID, projectID, serverID, teamID); err != nil {
		return err
	}
	v, err := s.Repos().DatabaseEnvVar().FindByID(ctx, id)
	if err != nil {
		return err
	}
	if v.DatabaseID != databaseID {
		return fiberutil.NotFound()
	}
	if err := s.Repos().DatabaseEnvVar().Delete(ctx, v.ID); err != nil {
		return err
	}
	s.BroadcastToTeam(teamID, "docker.database.env_var.deleted", map[string]any{
		"database_id": databaseID,
		"project_id":  projectID,
		"team_id":     teamID,
		"id":          v.ID,
		"key":         v.Key,
	})
	return nil
}
