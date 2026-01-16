package repositories

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/server/models"
)

// TaskRepository handles task database operations
type TaskRepository struct {
	BaseRepository
}

// NewTaskRepository creates a new TaskRepository instance
func NewTaskRepository(db *gorm.DB) *TaskRepository {
	return &TaskRepository{
		BaseRepository: NewBaseRepository(db),
	}
}

// Create creates a new task
func (r *TaskRepository) Create(ctx context.Context, task *models.Task) error {
	return r.DB().WithContext(ctx).Create(task).Error
}

// FindByID finds a task by ID
func (r *TaskRepository) FindByID(ctx context.Context, id string) (*models.Task, error) {
	var task models.Task
	err := r.DB().WithContext(ctx).First(&task, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTaskNotFound
		}
		return nil, err
	}
	return &task, nil
}

// FindByServer finds all tasks for a server with optional limit
func (r *TaskRepository) FindByServer(ctx context.Context, serverID string, limit int) ([]models.Task, error) {
	var tasks []models.Task
	query := r.DB().WithContext(ctx).
		Where("server_id = ?", serverID).
		Order("created_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	err := query.Find(&tasks).Error
	return tasks, err
}

// FindLatestByServer finds the latest task for a server
func (r *TaskRepository) FindLatestByServer(ctx context.Context, serverID string) (*models.Task, error) {
	var task models.Task
	err := r.DB().WithContext(ctx).
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

// Update updates a task
func (r *TaskRepository) Update(ctx context.Context, task *models.Task) error {
	return r.DB().WithContext(ctx).Save(task).Error
}
