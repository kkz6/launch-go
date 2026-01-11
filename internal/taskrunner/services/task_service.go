package services

import (
	"github.com/kkz6/launch-go/internal/taskrunner/handlers"
)

// TaskModel interface represents a task entity
type TaskModel interface {
	IsFinished() bool
}

// TaskRepository defines the data access contract
type TaskRepository interface {
	FindByID(id string) (TaskModel, error)
	UpdateStatus(id string, status string) error
	UpdateResult(id string, status string, exitCode int, output string) error
}

// TaskService handles task-related business logic and adapts repository to handler interface
type TaskService struct {
	repo TaskRepository
}

// NewTaskService creates a new task service
func NewTaskService(repo TaskRepository) *TaskService {
	return &TaskService{repo: repo}
}

// FindByID finds a task by ID
func (s *TaskService) FindByID(id string) (handlers.Task, error) {
	return s.repo.FindByID(id)
}

// UpdateStatus updates task status
func (s *TaskService) UpdateStatus(id string, status string) error {
	return s.repo.UpdateStatus(id, status)
}

// UpdateResult updates task result with status, exit code, and output
func (s *TaskService) UpdateResult(id string, status string, exitCode int, output string) error {
	return s.repo.UpdateResult(id, status, exitCode, output)
}

// Ensure TaskService implements handlers.TaskRepository
var _ handlers.TaskRepository = (*TaskService)(nil)
