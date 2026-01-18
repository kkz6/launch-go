package repositories

import (
	"context"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/script/models"
)

// ScriptExecutionRepository handles database operations for script executions
type ScriptExecutionRepository struct {
	DB *gorm.DB
}

// NewScriptExecutionRepository creates a new execution repository
func NewScriptExecutionRepository(db *gorm.DB) *ScriptExecutionRepository {
	return &ScriptExecutionRepository{DB: db}
}

// Create creates a new execution record
func (r *ScriptExecutionRepository) Create(ctx context.Context, execution *models.ScriptExecution) error {
	return r.DB.WithContext(ctx).Create(execution).Error
}

// FindByID finds an execution by ID
func (r *ScriptExecutionRepository) FindByID(ctx context.Context, id uint64) (*models.ScriptExecution, error) {
	var execution models.ScriptExecution

	err := r.DB.WithContext(ctx).
		Preload("Server").
		First(&execution, id).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrExecutionNotFound
		}

		return nil, err
	}

	return &execution, nil
}

// FindByScript finds all executions for a script
func (r *ScriptExecutionRepository) FindByScript(ctx context.Context, scriptID string) ([]models.ScriptExecution, error) {
	var executions []models.ScriptExecution

	err := r.DB.WithContext(ctx).
		Where("script_id = ?", scriptID).
		Preload("Server").
		Order("created_at DESC").
		Find(&executions).Error

	return executions, err
}

// FindByBatch finds all executions in a batch
func (r *ScriptExecutionRepository) FindByBatch(ctx context.Context, batchID string) ([]models.ScriptExecution, error) {
	var executions []models.ScriptExecution

	err := r.DB.WithContext(ctx).
		Where("batch_id = ?", batchID).
		Preload("Server").
		Order("created_at ASC").
		Find(&executions).Error

	return executions, err
}

// UpdateFields updates specific fields of an execution
func (r *ScriptExecutionRepository) UpdateFields(ctx context.Context, id uint64, fields map[string]any) error {
	return r.DB.WithContext(ctx).
		Model(&models.ScriptExecution{}).
		Where("id = ?", id).
		Updates(fields).Error
}
