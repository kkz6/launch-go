package repositories

import (
	"context"
	"errors"

	"gorm.io/gorm"

	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/pkg/repository"
)

// TaskRepository handles task database operations
type TaskRepository struct {
	repository.Base[models.Task]
}

// NewTaskRepository creates a new TaskRepository instance
func NewTaskRepository(db *gorm.DB) *TaskRepository {
	return &TaskRepository{
		Base: repository.NewBase[models.Task](db),
	}
}

// FindByID finds a task by ID
func (r *TaskRepository) FindByID(ctx context.Context, id string) (*models.Task, error) {
	task, err := r.Base.FindByID(ctx, id)
	if err != nil {
		if repository.IsNotFound(err) {
			return nil, fiberutil.NotFound()
		}
		return nil, err
	}
	return task, nil
}

// FindByServer finds all tasks for a server with optional limit
func (r *TaskRepository) FindByServer(ctx context.Context, serverID string, limit int) ([]models.Task, error) {
	var tasks []models.Task
	query := r.DB.WithContext(ctx).
		Where("server_id = ?", serverID).
		Order("created_at DESC")

	query = repository.ApplyFilters(query, repository.WithOptionalLimit(limit))

	err := query.Find(&tasks).Error
	return tasks, err
}

// FindLatestByServer finds the latest task for a server
func (r *TaskRepository) FindLatestByServer(ctx context.Context, serverID string) (*models.Task, error) {
	var task models.Task
	err := r.DB.WithContext(ctx).
		Where("server_id = ?", serverID).
		Order("created_at DESC").
		First(&task).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &task, nil
}

// UpdateOutput updates the output field of a task
func (r *TaskRepository) UpdateOutput(ctx context.Context, taskID string, output string) error {
	return r.Base.UpdateFields(ctx, taskID, map[string]interface{}{
		"output": output,
	})
}
