package repositories

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/server/models"
)

// CreateMetric creates a new metric
func (r *Repository) CreateMetric(ctx context.Context, metric *models.Metric) error {
	return create(r, ctx, metric)
}

// FindMetricsByServer finds metrics for a server with optional time range
func (r *Repository) FindMetricsByServer(ctx context.Context, serverID string, from, to *time.Time, limit int) ([]models.Metric, error) {
	var metrics []models.Metric
	query := r.db.WithContext(ctx).
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

// FindLatestMetricByServer finds the latest metric for a server
func (r *Repository) FindLatestMetricByServer(ctx context.Context, serverID string) (*models.Metric, error) {
	var metric models.Metric
	err := r.db.WithContext(ctx).
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

// DeleteOldMetrics deletes metrics older than a certain time
func (r *Repository) DeleteOldMetrics(ctx context.Context, serverID string, before time.Time) error {
	return r.db.WithContext(ctx).
		Where("server_id = ? AND recorded_at < ?", serverID, before).
		Delete(&models.Metric{}).Error
}
