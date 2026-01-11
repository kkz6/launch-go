package services

import (
	"context"

	"github.com/kkz6/launch-go/internal/modules/server/dto"
	"github.com/kkz6/launch-go/internal/modules/server/models"
)

// ListDatabases returns all databases on a server
func (s *Service) ListDatabases(ctx context.Context, serverID, teamID string) ([]models.Database, error) {
	if _, err := s.repo.FindServerByIDAndTeam(ctx, serverID, teamID); err != nil {
		return nil, err
	}

	return s.repo.FindDatabasesByServer(ctx, serverID)
}

// ListDatabaseUsers returns all database users on a server
func (s *Service) ListDatabaseUsers(ctx context.Context, serverID, teamID string) ([]models.DatabaseUser, error) {
	if _, err := s.repo.FindServerByIDAndTeam(ctx, serverID, teamID); err != nil {
		return nil, err
	}

	return s.repo.FindDatabaseUsersByServer(ctx, serverID)
}

// CreateDatabase creates a new database on a server
func (s *Service) CreateDatabase(ctx context.Context, serverID, teamID string, req *dto.CreateDatabaseRequest) (*models.Database, error) {
	_, err := s.repo.FindServerByIDAndTeam(ctx, serverID, teamID)
	if err != nil {
		return nil, err
	}

	db := &models.Database{
		ServerID: serverID,
		Name:     req.Name,
	}

	if err := s.repo.CreateDatabase(ctx, db); err != nil {
		return nil, err
	}

	return db, nil
}
