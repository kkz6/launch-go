package services

import (
	"context"

	"github.com/kkz6/launch-go/internal/modules/server/dto"
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
