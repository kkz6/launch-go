package repositories

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/server/models"
)

// CreateTask creates a new task
func (r *Repository) CreateTask(ctx context.Context, task *models.Task) error {
	return r.db.WithContext(ctx).Create(task).Error
}

// FindTaskByID finds a task by ID
func (r *Repository) FindTaskByID(ctx context.Context, id string) (*models.Task, error) {
	var task models.Task
	err := r.db.WithContext(ctx).First(&task, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTaskNotFound
		}

		return nil, err
	}

	return &task, nil
}

// FindTasksByServer finds all tasks for a server
func (r *Repository) FindTasksByServer(ctx context.Context, serverID string, limit int) ([]models.Task, error) {
	var tasks []models.Task
	query := r.db.WithContext(ctx).
		Where("server_id = ?", serverID).
		Order("created_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	err := query.Find(&tasks).Error

	return tasks, err
}

// FindLatestTaskByServer finds the latest task for a server
func (r *Repository) FindLatestTaskByServer(ctx context.Context, serverID string) (*models.Task, error) {
	var task models.Task
	err := r.db.WithContext(ctx).
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

// UpdateTask updates a task
func (r *Repository) UpdateTask(ctx context.Context, task *models.Task) error {
	return r.db.WithContext(ctx).Save(task).Error
}
