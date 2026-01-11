package services

import (
	"context"

	"github.com/kkz6/launch-go/internal/modules/server/dto"
)

// ListDatabases returns all databases on a server
func (s *Service) ListDatabases(ctx context.Context, serverID, teamID string) ([]dto.DatabaseResponse, error) {
	if _, err := s.repo.FindServerByIDAndTeam(ctx, serverID, teamID); err != nil {
		return nil, err
	}

	// TODO: Implement actual database listing via SSH or database service
	return []dto.DatabaseResponse{}, nil
}

// CreateDatabase creates a new database on a server
func (s *Service) CreateDatabase(ctx context.Context, serverID, teamID string, req *dto.CreateDatabaseRequest) (*dto.DatabaseResponse, error) {
	_, err := s.repo.FindServerByIDAndTeam(ctx, serverID, teamID)
	if err != nil {
		return nil, err
	}

	// TODO: Implement actual database creation via SSH or database service
	return &dto.DatabaseResponse{
		Name: req.Name,
	}, nil
}
