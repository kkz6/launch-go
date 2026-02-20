package services

import (
	"context"

	"github.com/kkz6/launch-go/internal/modules/server/models"
)

// ListTasks returns tasks for a server
func (s *Service) ListTasks(ctx context.Context, serverID, teamID string, limit int) ([]models.Task, error) {
	if _, err := s.repos.Server().FindByIDAndTeam(ctx, serverID, teamID); err != nil {
		return nil, err
	}

	return s.repos.Task().FindByServer(ctx, serverID, limit)
}

// GetTask returns a specific task by ID for a server
func (s *Service) GetTask(ctx context.Context, taskID, serverID, teamID string) (*models.Task, error) {
	if _, err := s.repos.Server().FindByIDAndTeam(ctx, serverID, teamID); err != nil {
		return nil, err
	}

	return s.repos.Task().FindByID(ctx, taskID)
}

// GetLatestTask returns the latest task for a server
func (s *Service) GetLatestTask(ctx context.Context, serverID, teamID string) (*models.Task, error) {
	if _, err := s.repos.Server().FindByIDAndTeam(ctx, serverID, teamID); err != nil {
		return nil, err
	}

	return s.repos.Task().FindLatestByServer(ctx, serverID)
}
