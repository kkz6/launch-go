package services

import (
	"context"
	"strings"

	"github.com/kkz6/launch-go/internal/modules/docker/dto"
	"github.com/kkz6/launch-go/internal/modules/docker/models"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// RegistryCredentialService manages saved docker-registry logins.
// Team-scoped CRUD with encryption-at-rest for both username and
// password (the latter never leaves the service layer; the former
// is decrypted into the response only when a caller asks).
type RegistryCredentialService struct {
	*BaseService
}

func NewRegistryCredentialService(deps *ServiceDeps) *RegistryCredentialService {
	return &RegistryCredentialService{BaseService: NewBaseService(deps)}
}

// ListCredentials returns every credential the team owns. Used by
// Settings → Connections + the workload-create pickers.
func (s *RegistryCredentialService) ListCredentials(
	ctx context.Context, teamID string,
) ([]dto.RegistryCredentialResponse, error) {
	rows, err := s.Repos().RegistryCredential().ListForTeam(ctx, teamID)
	if err != nil {
		return nil, err
	}
	out := make([]dto.RegistryCredentialResponse, 0, len(rows))
	for i := range rows {
		out = append(out, *dto.ToRegistryCredentialResponse(&rows[i]))
	}
	return out, nil
}

// CreateCredential persists a new saved login. Username + password
// are written through `dbtype.EncryptedString` so the values land
// encrypted at rest.
//
// Per-team name uniqueness is checked here (cheap) and also enforced
// by the DB index — the dual check is defensive: races between two
// concurrent creates with the same name lose at the DB layer with a
// generic duplicate-key error; this lets us return the friendlier
// 409 on the common case.
func (s *RegistryCredentialService) CreateCredential(
	ctx context.Context, teamID, userID string,
	req *dto.CreateRegistryCredentialRequest,
) (dto.RegistryCredentialResponse, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return dto.RegistryCredentialResponse{}, fiberutil.BadRequest("Name is required")
	}

	taken, err := s.Repos().RegistryCredential().ExistsByName(ctx, teamID, name, "")
	if err != nil {
		return dto.RegistryCredentialResponse{}, err
	}
	if taken {
		return dto.RegistryCredentialResponse{}, fiberutil.Conflict(
			"A registry credential with that name already exists",
		)
	}

	// Normalize the registry URL — empty → nil so the deploy script
	// can branch on "no URL means Docker Hub" without dealing with
	// the empty-string variant.
	var registryURL *string
	if req.RegistryURL != nil {
		u := strings.TrimSpace(*req.RegistryURL)
		if u != "" {
			registryURL = &u
		}
	}

	cred := &models.RegistryCredential{
		TeamID:      teamID,
		UserID:      &userID,
		Name:        name,
		RegistryURL: registryURL,
		Username:    dbtype.EncryptedString(strings.TrimSpace(req.Username)),
		Password:    dbtype.EncryptedString(req.Password),
	}
	if err := s.Repos().RegistryCredential().Create(ctx, cred); err != nil {
		return dto.RegistryCredentialResponse{}, err
	}

	s.BroadcastToTeam(teamID, "docker.registry_credential.added", map[string]any{
		"id":      cred.ID,
		"team_id": teamID,
		"name":    cred.Name,
	})
	return *dto.ToRegistryCredentialResponse(cred), nil
}

// UpdateCredential applies partial changes. nil fields leave the
// stored value alone — empty string is rejected for username +
// password (would create a broken credential).
func (s *RegistryCredentialService) UpdateCredential(
	ctx context.Context, id, teamID, userID string,
	req *dto.UpdateRegistryCredentialRequest,
) (dto.RegistryCredentialResponse, error) {
	_ = userID
	cred, err := s.Repos().RegistryCredential().FindByIDForTeam(ctx, id, teamID)
	if err != nil {
		return dto.RegistryCredentialResponse{}, err
	}

	updates := map[string]any{}
	if req.Name != nil {
		newName := strings.TrimSpace(*req.Name)
		if newName == "" {
			return dto.RegistryCredentialResponse{}, fiberutil.BadRequest("Name cannot be empty")
		}
		if newName != cred.Name {
			taken, err := s.Repos().RegistryCredential().ExistsByName(ctx, teamID, newName, id)
			if err != nil {
				return dto.RegistryCredentialResponse{}, err
			}
			if taken {
				return dto.RegistryCredentialResponse{}, fiberutil.Conflict(
					"A registry credential with that name already exists",
				)
			}
			updates["name"] = newName
		}
	}
	if req.RegistryURL != nil {
		u := strings.TrimSpace(*req.RegistryURL)
		if u == "" {
			// Caller explicitly clears the URL → Docker Hub.
			updates["registry_url"] = nil
		} else {
			updates["registry_url"] = u
		}
	}
	if req.Username != nil {
		u := strings.TrimSpace(*req.Username)
		if u == "" {
			return dto.RegistryCredentialResponse{}, fiberutil.BadRequest("Username cannot be empty")
		}
		updates["username"] = dbtype.EncryptedString(u)
	}
	if req.Password != nil {
		if *req.Password == "" {
			return dto.RegistryCredentialResponse{}, fiberutil.BadRequest("Password cannot be empty")
		}
		updates["password"] = dbtype.EncryptedString(*req.Password)
	}

	if len(updates) > 0 {
		if err := s.Repos().RegistryCredential().UpdateFields(ctx, id, updates); err != nil {
			return dto.RegistryCredentialResponse{}, err
		}
	}
	reloaded, err := s.Repos().RegistryCredential().FindByIDForTeam(ctx, id, teamID)
	if err != nil {
		return dto.RegistryCredentialResponse{}, err
	}
	s.BroadcastToTeam(teamID, "docker.registry_credential.updated", map[string]any{
		"id":      id,
		"team_id": teamID,
		"name":    reloaded.Name,
	})
	return *dto.ToRegistryCredentialResponse(reloaded), nil
}

// DeleteCredential soft-deletes the row. The FK on docker_applications
// is ON DELETE SET NULL (see migration 0037) so applications that
// reference this credential get disconnected, not tombstoned —
// they'll need a re-attach or inline credentials on the next deploy.
// The compose join is ON DELETE CASCADE so attached compose stacks
// silently drop the link.
func (s *RegistryCredentialService) DeleteCredential(
	ctx context.Context, id, teamID, userID string,
) error {
	_ = userID
	cred, err := s.Repos().RegistryCredential().FindByIDForTeam(ctx, id, teamID)
	if err != nil {
		return err
	}
	if err := s.Repos().RegistryCredential().Delete(ctx, id); err != nil {
		return err
	}
	s.BroadcastToTeam(teamID, "docker.registry_credential.deleted", map[string]any{
		"id":      id,
		"team_id": teamID,
		"name":    cred.Name,
	})
	return nil
}
