package services

import (
	"context"
	"encoding/json"

	"github.com/kkz6/launch-go/internal/modules/server/dto"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/types"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// ListServerProviders returns all server providers for a team. Signature
// matches IndexFunc.
func (s *Service) ListServerProviders(ctx context.Context, teamID string) ([]dto.ServerProviderResponse, error) {
	providers, err := s.repos.ServerProvider().FindByTeam(ctx, teamID)
	if err != nil {
		return nil, err
	}
	out := make([]dto.ServerProviderResponse, len(providers))
	for i := range providers {
		out[i] = dto.ToServerProviderResponse(&providers[i])
	}
	return out, nil
}

// CreateServerProvider connects a new cloud provider account for the team.
// Credentials are normalised into the map shape the providers/ package's
// Extract* helpers expect (e.g. {"token": ...} for token providers, AWS keys
// for AWS), then JSON-encoded and stored encrypted.
//
// Signature matches CreateFunc[CreateServerProviderRequest, ServerProviderResponse].
func (s *Service) CreateServerProvider(ctx context.Context, teamID, userID string, req *dto.CreateServerProviderRequest) (dto.ServerProviderResponse, error) {
	providerType, err := types.ParseServerProvider(req.Provider)
	if err != nil {
		return dto.ServerProviderResponse{}, ErrInvalidProvider
	}

	creds, err := buildCredentialsMap(providerType, req)
	if err != nil {
		return dto.ServerProviderResponse{}, err
	}

	credsJSON, err := json.Marshal(creds)
	if err != nil {
		return dto.ServerProviderResponse{}, fiberutil.Internal("Failed to serialize credentials")
	}

	teamIDCopy := teamID
	provider := &models.ServerProvider{
		UserID:      userID,
		TeamID:      &teamIDCopy,
		Profile:     &req.Profile,
		Provider:    providerType,
		Credentials: dbtype.EncryptedString(credsJSON),
		Connected:   true,
	}

	if err := s.repos.ServerProvider().Create(ctx, provider); err != nil {
		return dto.ServerProviderResponse{}, err
	}

	return dto.ToServerProviderResponse(provider), nil
}

// DeleteServerProvider removes a connected provider, scoped to the team.
// Signature matches DeleteFunc.
func (s *Service) DeleteServerProvider(ctx context.Context, id, teamID, _ string) error {
	provider, err := s.repos.ServerProvider().FindByID(ctx, id)
	if err != nil {
		return err
	}
	if provider.TeamID == nil || *provider.TeamID != teamID {
		return fiberutil.NotFound()
	}
	return s.repos.ServerProvider().Delete(ctx, id)
}

// buildCredentialsMap normalises the per-provider request fields into the
// `map[string]any` shape that the providers/ package's Extract* helpers
// consume. The UI sends `api_token`, but ExtractToken reads `token` — this is
// where the translation happens.
func buildCredentialsMap(providerType types.ServerProvider, req *dto.CreateServerProviderRequest) (map[string]any, error) {
	if providerType == types.ProviderAWS {
		if req.AccessKey == "" || req.SecretKey == "" || req.Region == "" {
			return nil, fiberutil.BadRequest("access_key, secret_key and region are required for AWS")
		}
		return map[string]any{
			"access_key": req.AccessKey,
			"secret_key": req.SecretKey,
			"region":     req.Region,
		}, nil
	}

	if req.APIToken == "" {
		return nil, fiberutil.BadRequest("api_token is required for this provider")
	}
	return map[string]any{"token": req.APIToken}, nil
}
