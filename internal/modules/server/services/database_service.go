package services

import (
	"context"
	"fmt"

	dbjobs "github.com/kkz6/launch-go/internal/modules/database/jobs"
	dbmodels "github.com/kkz6/launch-go/internal/modules/database/models"
	"github.com/kkz6/launch-go/internal/modules/server/dto"
)

// ListDatabases returns all databases on a server
func (s *Service) ListDatabases(ctx context.Context, serverID, teamID string) ([]dbmodels.Database, error) {
	if _, err := s.repo.FindServerByIDAndTeam(ctx, serverID, teamID); err != nil {
		return nil, err
	}

	return s.repo.FindDatabasesByServer(ctx, serverID)
}

// ListDatabaseUsers returns all database users on a server
func (s *Service) ListDatabaseUsers(ctx context.Context, serverID, teamID string) ([]dbmodels.DatabaseUser, error) {
	if _, err := s.repo.FindServerByIDAndTeam(ctx, serverID, teamID); err != nil {
		return nil, err
	}

	return s.repo.FindDatabaseUsersByServer(ctx, serverID)
}

// CreateDatabase creates a new database on a server
func (s *Service) CreateDatabase(ctx context.Context, serverID, teamID string, req *dto.CreateDatabaseRequest) (*dbmodels.Database, error) {
	_, err := s.repo.FindServerByIDAndTeam(ctx, serverID, teamID)
	if err != nil {
		return nil, err
	}

	db := &dbmodels.Database{
		ServerID: serverID,
		Name:     req.Name,
	}

	if err := s.repo.CreateDatabase(ctx, db); err != nil {
		return nil, err
	}

	return db, nil
}

// SyncDatabases syncs databases from the server
func (s *Service) SyncDatabases(ctx context.Context, serverID, teamID string, userID *string) error {
	// Verify server belongs to team
	if _, err := s.repo.FindServerByIDAndTeam(ctx, serverID, teamID); err != nil {
		return err
	}

	if s.queue == nil {
		return ErrQueueNotConfigured
	}

	task, err := dbjobs.NewSyncDatabasesTask(serverID, userID)
	if err != nil {
		return fmt.Errorf("failed to create sync task: %w", err)
	}

	if _, err := s.queue.EnqueueDefault(task); err != nil {
		s.logger.Error().Err(err).Str("server_id", serverID).Msg("Failed to enqueue sync databases job")
		return fmt.Errorf("failed to enqueue sync job: %w", err)
	}

	return nil
}
