package services

import (
	"context"
	"fmt"

	"github.com/kkz6/launch-go/internal/modules/dockerregistry/contracts"
	"github.com/kkz6/launch-go/internal/modules/dockerregistry/dto"
	"github.com/kkz6/launch-go/internal/modules/dockerregistry/models"
	"github.com/kkz6/launch-go/internal/modules/dockerregistry/types"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/launch/activity"
	"github.com/kkz6/launch-go/internal/pkg/service"
)

// ErrNameTaken is returned when a credential with the same name already
// exists in the team.
var ErrNameTaken = fiberutil.Conflict("A docker registry credential with this name already exists")

// ServiceDeps bundles dependencies the service needs.
type ServiceDeps struct {
	service.Dependencies
	Repos contracts.RepositoryRegistry
}

// Service holds the business logic for docker registry credentials.
type Service struct {
	service.Base
	repos contracts.RepositoryRegistry
}

// NewService constructs a service from its dependencies.
func NewService(deps ServiceDeps) *Service {
	return &Service{
		Base:  service.NewBaseFromDeps(deps.Dependencies),
		repos: deps.Repos,
	}
}

// Repos returns the repository registry.
func (s *Service) Repos() contracts.RepositoryRegistry { return s.repos }

// List returns every credential for the team.
// Signature matches IndexFunc: (ctx, teamID).
func (s *Service) List(ctx context.Context, teamID string) ([]dto.CredentialResponse, error) {
	creds, err := s.repos.Credential().FindByTeam(ctx, teamID)
	if err != nil {
		return nil, err
	}
	return dto.ToCredentialResponseList(creds), nil
}

// Get returns a single credential by ID, scoped to the team.
// Signature matches ShowFunc: (ctx, id, teamID).
func (s *Service) Get(ctx context.Context, id, teamID string) (dto.CredentialResponse, error) {
	c, err := s.repos.Credential().FindByIDAndTeam(ctx, id, teamID)
	if err != nil {
		return dto.CredentialResponse{}, err
	}
	return dto.ToCredentialResponse(c), nil
}

// Create persists a new credential.
// Signature matches CreateFunc: (ctx, teamID, userID, req).
func (s *Service) Create(ctx context.Context, teamID, userID string, req *dto.CreateCredentialRequest) (dto.CredentialResponse, error) {
	if !req.Type.IsValid() {
		return dto.CredentialResponse{}, fiberutil.BadRequest(fmt.Sprintf("Invalid registry type: %s", req.Type))
	}

	url := req.URL
	if url == "" {
		url = req.Type.DefaultURL()
	}
	if url == "" {
		return dto.CredentialResponse{}, fiberutil.BadRequest("URL is required for generic registries")
	}

	if existing, err := s.repos.Credential().FindByNameAndTeam(ctx, req.Name, teamID); err != nil {
		return dto.CredentialResponse{}, err
	} else if existing != nil {
		return dto.CredentialResponse{}, ErrNameTaken
	}

	c := &models.Credential{
		Name:     req.Name,
		Type:     req.Type,
		URL:      url,
		Username: dbtype.EncryptedString(req.Username),
		Password: dbtype.EncryptedString(req.Password),
	}
	c.TeamID = teamID

	if err := s.repos.Credential().Create(ctx, c); err != nil {
		return dto.CredentialResponse{}, fmt.Errorf("failed to create docker registry credential: %w", err)
	}

	activity.RecordEvent(ctx, "created", userID, c, "Docker registry credential was created")
	return dto.ToCredentialResponse(c), nil
}

// Update applies partial changes to a credential.
// Signature matches UpdateFunc: (ctx, id, teamID, userID, req).
func (s *Service) Update(ctx context.Context, id, teamID, userID string, req *dto.UpdateCredentialRequest) (dto.CredentialResponse, error) {
	c, err := s.repos.Credential().FindByIDAndTeam(ctx, id, teamID)
	if err != nil {
		return dto.CredentialResponse{}, err
	}

	updates := map[string]any{}
	if req.Name != "" && req.Name != c.Name {
		// Reject duplicate names within the team.
		if existing, err := s.repos.Credential().FindByNameAndTeam(ctx, req.Name, teamID); err != nil {
			return dto.CredentialResponse{}, err
		} else if existing != nil && existing.ID != c.ID {
			return dto.CredentialResponse{}, ErrNameTaken
		}
		updates["name"] = req.Name
	}
	if req.URL != "" {
		updates["url"] = req.URL
	}
	if req.Username != "" {
		updates["username"] = dbtype.EncryptedString(req.Username)
	}
	if req.Password != "" {
		// Empty password means "leave unchanged" — we never want to wipe a
		// stored password by accident from a partial UI update.
		updates["password"] = dbtype.EncryptedString(req.Password)
	}

	if len(updates) > 0 {
		if err := s.repos.Credential().Update(ctx, c.ID, updates); err != nil {
			return dto.CredentialResponse{}, fmt.Errorf("failed to update docker registry credential: %w", err)
		}
	}

	c, err = s.repos.Credential().FindByIDAndTeam(ctx, id, teamID)
	if err != nil {
		return dto.CredentialResponse{}, err
	}

	activity.RecordEvent(ctx, "updated", userID, c, "Docker registry credential was updated")
	return dto.ToCredentialResponse(c), nil
}

// Delete removes a credential.
// Signature matches DeleteFunc: (ctx, id, teamID, userID).
func (s *Service) Delete(ctx context.Context, id, teamID, userID string) error {
	c, err := s.repos.Credential().FindByIDAndTeam(ctx, id, teamID)
	if err != nil {
		return err
	}
	if err := s.repos.Credential().Delete(ctx, c.ID); err != nil {
		return fmt.Errorf("failed to delete docker registry credential: %w", err)
	}
	activity.RecordEvent(ctx, "deleted", userID, c, "Docker registry credential was deleted")
	return nil
}

// GetForApp returns the decrypted credential for use during a deploy. The
// caller is responsible for not exposing the password back through the API.
// Currently unused but lives here so the future application module has a
// stable entrypoint without reaching into the repository layer.
func (s *Service) GetForApp(ctx context.Context, id, teamID string) (*models.Credential, error) {
	_ = types.AllTypes // keep dependency for compile parity
	return s.repos.Credential().FindByIDAndTeam(ctx, id, teamID)
}
