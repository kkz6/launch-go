package services

import (
	"context"
	"regexp"
	"strings"

	"github.com/kkz6/launch-go/internal/modules/docker/dto"
	"github.com/kkz6/launch-go/internal/modules/docker/models"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// EnvVarService manages env vars on a docker application. Mutations
// don't restart the container — the next deploy applies them. The UI
// surfaces this with a "Pending — redeploy to apply" badge.
type EnvVarService struct {
	*BaseService
}

func NewEnvVarService(deps *ServiceDeps) *EnvVarService {
	return &EnvVarService{BaseService: NewBaseService(deps)}
}

// ListEnvVars returns env vars for an application. Secret values are
// masked. Pass reveal=true to GetEnvVar to see one in cleartext.
func (s *EnvVarService) ListEnvVars(
	ctx context.Context, applicationID, projectID, serverID, teamID string,
) ([]dto.EnvVarResponse, error) {
	if _, err := s.scopedAppForChild(ctx, applicationID, projectID, serverID, teamID); err != nil {
		return nil, err
	}
	rows, err := s.Repos().EnvVar().ListForApplication(ctx, applicationID)
	if err != nil {
		return nil, err
	}
	out := make([]dto.EnvVarResponse, 0, len(rows))
	for i := range rows {
		out = append(out, *dto.ToEnvVarResponse(&rows[i], false))
	}
	return out, nil
}

// CreateEnvVar adds a key/value to the application. Key uniqueness is
// enforced per-app; we surface a 409 before the DB index throws.
func (s *EnvVarService) CreateEnvVar(
	ctx context.Context, applicationID, projectID, serverID, teamID, userID string,
	req *dto.CreateEnvVarRequest,
) (dto.EnvVarResponse, error) {
	_ = userID
	app, err := s.scopedAppForChild(ctx, applicationID, projectID, serverID, teamID)
	if err != nil {
		return dto.EnvVarResponse{}, err
	}

	key := strings.TrimSpace(req.Key)
	if !validEnvVarKey.MatchString(key) {
		return dto.EnvVarResponse{}, fiberutil.BadRequest(
			"Env var key must start with a letter or underscore and contain only letters, digits, or underscores",
		)
	}
	taken, err := s.Repos().EnvVar().ExistsByKey(ctx, applicationID, key, "")
	if err != nil {
		return dto.EnvVarResponse{}, err
	}
	if taken {
		return dto.EnvVarResponse{}, fiberutil.Conflict(
			"An env var with that key already exists on this application",
		)
	}

	v := &models.ApplicationEnvVar{
		ApplicationID: applicationID,
		Key:           key,
		Value:         req.Value,
		IsSecret:      req.IsSecret,
	}
	if err := s.Repos().EnvVar().Create(ctx, v); err != nil {
		return dto.EnvVarResponse{}, err
	}

	resp := dto.ToEnvVarResponse(v, false)
	s.BroadcastToTeam(teamID, "docker.application.env_var.added", map[string]any{
		"application_id": app.ID,
		"server_id":      app.ServerID,
		"team_id":        app.TeamID,
		"env_var_id":     v.ID,
	})
	return *resp, nil
}

// UpdateEnvVar mutates value/is_secret. Key is immutable — remove +
// add to change a key so old/new keys don't briefly coexist.
func (s *EnvVarService) UpdateEnvVar(
	ctx context.Context, id, applicationID, projectID, serverID, teamID, userID string,
	req *dto.UpdateEnvVarRequest,
) (dto.EnvVarResponse, error) {
	_ = userID
	app, err := s.scopedAppForChild(ctx, applicationID, projectID, serverID, teamID)
	if err != nil {
		return dto.EnvVarResponse{}, err
	}
	v, err := s.Repos().EnvVar().FindByID(ctx, id)
	if err != nil {
		return dto.EnvVarResponse{}, err
	}
	if v.ApplicationID != applicationID {
		return dto.EnvVarResponse{}, fiberutil.NotFound()
	}

	updates := map[string]any{}
	if req.Value != nil {
		updates["value"] = *req.Value
	}
	if req.IsSecret != nil {
		updates["is_secret"] = *req.IsSecret
	}
	if len(updates) > 0 {
		if err := s.Repos().EnvVar().UpdateFields(ctx, id, updates); err != nil {
			return dto.EnvVarResponse{}, err
		}
	}
	reloaded, err := s.Repos().EnvVar().FindByID(ctx, id)
	if err != nil {
		return dto.EnvVarResponse{}, err
	}
	s.BroadcastToTeam(teamID, "docker.application.env_var.updated", map[string]any{
		"application_id": app.ID,
		"server_id":      app.ServerID,
		"team_id":        app.TeamID,
		"env_var_id":     id,
	})
	return *dto.ToEnvVarResponse(reloaded, false), nil
}

// DeleteEnvVar soft-deletes the row.
func (s *EnvVarService) DeleteEnvVar(
	ctx context.Context, id, applicationID, projectID, serverID, teamID, userID string,
) error {
	_ = userID
	app, err := s.scopedAppForChild(ctx, applicationID, projectID, serverID, teamID)
	if err != nil {
		return err
	}
	v, err := s.Repos().EnvVar().FindByID(ctx, id)
	if err != nil {
		return err
	}
	if v.ApplicationID != applicationID {
		return fiberutil.NotFound()
	}
	if err := s.Repos().EnvVar().Delete(ctx, id); err != nil {
		return err
	}
	s.BroadcastToTeam(teamID, "docker.application.env_var.deleted", map[string]any{
		"application_id": app.ID,
		"server_id":      app.ServerID,
		"team_id":        app.TeamID,
		"env_var_id":     id,
	})
	return nil
}

// SetEnvVars replaces the entire env-var set in a single transaction.
// Backs the "paste a .env" workflow. Soft-deletes the existing vars
// and inserts the new ones so a paste of an empty list clears
// everything.
func (s *EnvVarService) SetEnvVars(
	ctx context.Context, applicationID, projectID, serverID, teamID, userID string,
	req *dto.SetEnvVarsRequest,
) ([]dto.EnvVarResponse, error) {
	_ = userID
	app, err := s.scopedAppForChild(ctx, applicationID, projectID, serverID, teamID)
	if err != nil {
		return nil, err
	}

	// Validate keys before touching the DB — we don't want a partial
	// write where half the new vars made it in and the rest failed
	// validation.
	for _, e := range req.Vars {
		k := strings.TrimSpace(e.Key)
		if !validEnvVarKey.MatchString(k) {
			return nil, fiberutil.BadRequest("Invalid env var key: " + k)
		}
	}
	seen := make(map[string]bool, len(req.Vars))
	for _, e := range req.Vars {
		k := strings.TrimSpace(e.Key)
		if seen[k] {
			return nil, fiberutil.BadRequest("Duplicate env var key: " + k)
		}
		seen[k] = true
	}

	tx := s.Repos().EnvVar().DB.WithContext(ctx).Begin()
	if tx.Error != nil {
		return nil, tx.Error
	}
	// Soft-delete existing rows for this app — we can't use a single
	// UPDATE because we want the gorm deleted_at index to apply.
	if err := tx.Where("application_id = ?", applicationID).
		Delete(&models.ApplicationEnvVar{}).Error; err != nil {
		tx.Rollback()
		return nil, err
	}
	for _, e := range req.Vars {
		row := &models.ApplicationEnvVar{
			ApplicationID: applicationID,
			Key:           strings.TrimSpace(e.Key),
			Value:         e.Value,
			IsSecret:      e.IsSecret,
		}
		if err := tx.Create(row).Error; err != nil {
			tx.Rollback()
			return nil, err
		}
	}
	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	rows, err := s.Repos().EnvVar().ListForApplication(ctx, applicationID)
	if err != nil {
		return nil, err
	}
	out := make([]dto.EnvVarResponse, 0, len(rows))
	for i := range rows {
		out = append(out, *dto.ToEnvVarResponse(&rows[i], false))
	}
	s.BroadcastToTeam(teamID, "docker.application.env_var.set", map[string]any{
		"application_id": app.ID,
		"server_id":      app.ServerID,
		"team_id":        app.TeamID,
		"count":          len(rows),
	})
	return out, nil
}

// scopedAppForChild validates the project + application chain and
// returns the application so the caller can broadcast routing fields.
func (s *EnvVarService) scopedAppForChild(
	ctx context.Context, applicationID, projectID, serverID, teamID string,
) (*models.Application, error) {
	if _, err := s.Repos().Project().FindByIDAndTeamServer(ctx, projectID, teamID, serverID); err != nil {
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

// validEnvVarKey enforces POSIX-ish env-name rules. Letters/digits/
// underscores, can't start with a digit. Hyphens are technically legal
// in some shells but break sourcing .env files, so we reject them.
var validEnvVarKey = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
