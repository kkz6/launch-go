package services

import (
	"context"

	"github.com/kkz6/launch-go/internal/modules/server/models"
)

// ListServerProviders returns all server providers for a team
func (s *Service) ListServerProviders(ctx context.Context, teamID string) ([]models.ServerProvider, error) {
	return s.repos.ServerProvider().FindByTeam(ctx, teamID)
}
