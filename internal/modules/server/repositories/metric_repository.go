package repositories

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/pkg/repository"
)

// MetricRepository handles metric database operations
type MetricRepository struct {
	repository.Base[models.Metric]
}

// NewMetricRepository creates a new MetricRepository instance
func NewMetricRepository(db *gorm.DB) *MetricRepository {
	return &MetricRepository{
		Base: repository.NewBase[models.Metric](db),
	}
}

// FindByServer finds metrics for a server with optional time range
func (r *MetricRepository) FindByServer(ctx context.Context, serverID string, from, to *time.Time, limit int) ([]models.Metric, error) {
	var metrics []models.Metric
	query := r.DB.WithContext(ctx).Where("server_id = ?", serverID)

	query = repository.ApplyFilters(query,
		repository.WithDateRange("recorded_at", from, to),
		repository.WithOptionalLimit(limit),
	)

	err := query.Order("recorded_at DESC").Find(&metrics).Error
	return metrics, err
}

// FindLatestByServer finds the latest metric for a server
func (r *MetricRepository) FindLatestByServer(ctx context.Context, serverID string) (*models.Metric, error) {
	return repository.FindOneOrNil[models.Metric](ctx, r.DB,
		repository.WithServerID(serverID),
		repository.OrderBy("recorded_at", "DESC"),
	)
}

// DeleteOld deletes metrics older than a certain time for a specific server
func (r *MetricRepository) DeleteOld(ctx context.Context, serverID string, before time.Time) error {
	return r.DB.WithContext(ctx).
		Where("server_id = ? AND recorded_at < ?", serverID, before).
		Delete(&models.Metric{}).Error
}

// DeleteOlderThan deletes all metrics older than a certain time across all servers
// Returns the number of deleted records
func (r *MetricRepository) DeleteOlderThan(ctx context.Context, before time.Time) (int64, error) {
	result := r.DB.WithContext(ctx).
		Where("created_at < ?", before).
		Delete(&models.Metric{})
	return result.RowsAffected, result.Error
}
