package services

import (
	"context"

	"github.com/kkz6/launch-go/internal/modules/server/dto"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// ListTasks returns tasks for a server. Carries an extra `limit` query
// param so it does not fit IndexNested directly; routes wire a small
// bespoke handler.
func (s *Service) ListTasks(ctx context.Context, serverID, teamID string, limit int) ([]dto.TaskResponse, error) {
	if _, err := s.repos.Server().FindByIDAndTeam(ctx, serverID, teamID); err != nil {
		return nil, err
	}
	tasks, err := s.repos.Task().FindByServer(ctx, serverID, limit)
	if err != nil {
		return nil, err
	}
	out := make([]dto.TaskResponse, len(tasks))
	for i := range tasks {
		out[i] = dto.ToTaskResponse(&tasks[i])
	}
	return out, nil
}

// GetTask returns a specific task by ID for a server. Signature matches
// ShowNestedFunc: (ctx, id, parentID, teamID).
func (s *Service) GetTask(ctx context.Context, taskID, serverID, teamID string) (dto.TaskResponse, error) {
	if _, err := s.repos.Server().FindByIDAndTeam(ctx, serverID, teamID); err != nil {
		return dto.TaskResponse{}, err
	}
	task, err := s.repos.Task().FindByID(ctx, taskID)
	if err != nil {
		return dto.TaskResponse{}, err
	}
	if task == nil {
		return dto.TaskResponse{}, fiberutil.NotFound("Task not found")
	}
	return dto.ToTaskResponse(task), nil
}

// GetLatestTaskRaw returns the latest task model. Used by callers that
// need the raw model (e.g. provision-status composition).
func (s *Service) GetLatestTaskRaw(ctx context.Context, serverID, teamID string) (*models.Task, error) {
	if _, err := s.repos.Server().FindByIDAndTeam(ctx, serverID, teamID); err != nil {
		return nil, err
	}
	return s.repos.Task().FindLatestByServer(ctx, serverID)
}

// GetLatestTask returns the latest task for a server. Signature matches
// IndexNestedFunc.
func (s *Service) GetLatestTask(ctx context.Context, serverID, teamID string) (dto.TaskResponse, error) {
	if _, err := s.repos.Server().FindByIDAndTeam(ctx, serverID, teamID); err != nil {
		return dto.TaskResponse{}, err
	}
	task, err := s.repos.Task().FindLatestByServer(ctx, serverID)
	if err != nil {
		return dto.TaskResponse{}, err
	}
	if task == nil {
		return dto.TaskResponse{}, fiberutil.NotFound("No tasks found")
	}
	return dto.ToTaskResponse(task), nil
}
