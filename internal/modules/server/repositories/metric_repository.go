package repositories

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/server/models"
)

// MetricRepository handles metric database operations
type MetricRepository struct {
	BaseRepository
}

// NewMetricRepository creates a new MetricRepository instance
func NewMetricRepository(db *gorm.DB) *MetricRepository {
	return &MetricRepository{
		BaseRepository: NewBaseRepository(db),
	}
}

// Create creates a new metric
func (r *MetricRepository) Create(ctx context.Context, metric *models.Metric) error {
	return r.DB().WithContext(ctx).Create(metric).Error
}

// FindByServer finds metrics for a server with optional time range
func (r *MetricRepository) FindByServer(ctx context.Context, serverID string, from, to *time.Time, limit int) ([]models.Metric, error) {
	var metrics []models.Metric
	query := r.DB().WithContext(ctx).
		Where("server_id = ?", serverID)

	if from != nil {
		query = query.Where("recorded_at >= ?", from)
	}

	if to != nil {
		query = query.Where("recorded_at <= ?", to)
	}

	query = query.Order("recorded_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	err := query.Find(&metrics).Error
	return metrics, err
}

// FindLatestByServer finds the latest metric for a server
func (r *MetricRepository) FindLatestByServer(ctx context.Context, serverID string) (*models.Metric, error) {
	var metric models.Metric
	err := r.DB().WithContext(ctx).
		Where("server_id = ?", serverID).
		Order("recorded_at DESC").
		First(&metric).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &metric, nil
}

// DeleteOld deletes metrics older than a certain time
func (r *MetricRepository) DeleteOld(ctx context.Context, serverID string, before time.Time) error {
	return r.DB().WithContext(ctx).
		Where("server_id = ? AND recorded_at < ?", serverID, before).
		Delete(&models.Metric{}).Error
}
