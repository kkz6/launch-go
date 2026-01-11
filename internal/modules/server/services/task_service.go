package services

import (
	"context"

	"github.com/kkz6/launch-go/internal/modules/server/models"
)

// ListTasks returns tasks for a server
func (s *Service) ListTasks(ctx context.Context, serverID, teamID string, limit int) ([]models.Task, error) {
	if _, err := s.repo.FindServerByIDAndTeam(ctx, serverID, teamID); err != nil {
		return nil, err
	}

	return s.repo.FindTasksByServer(ctx, serverID, limit)
}

// GetLatestTask returns the latest task for a server
func (s *Service) GetLatestTask(ctx context.Context, serverID, teamID string) (*models.Task, error) {
	if _, err := s.repo.FindServerByIDAndTeam(ctx, serverID, teamID); err != nil {
		return nil, err
	}

	return s.repo.FindLatestTaskByServer(ctx, serverID)
}
