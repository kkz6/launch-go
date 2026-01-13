package activity

import (
	"context"

	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) FindByID(ctx context.Context, id string) (*ActivityLog, error) {
	var activity ActivityLog
	if err := r.db.WithContext(ctx).First(&activity, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &activity, nil
}

func (r *Repository) FindBySubject(ctx context.Context, subjectType, subjectID string) ([]ActivityLog, error) {
	var activities []ActivityLog
	err := r.db.WithContext(ctx).
		Where("subject_type = ? AND subject_id = ?", subjectType, subjectID).
		Order("created_at DESC").
		Find(&activities).Error
	return activities, err
}

func (r *Repository) FindByCauser(ctx context.Context, causerType, causerID string) ([]ActivityLog, error) {
	var activities []ActivityLog
	err := r.db.WithContext(ctx).
		Where("causer_type = ? AND causer_id = ?", causerType, causerID).
		Order("created_at DESC").
		Find(&activities).Error
	return activities, err
}

func (r *Repository) FindByLogName(ctx context.Context, logName string, limit int) ([]ActivityLog, error) {
	var activities []ActivityLog
	query := r.db.WithContext(ctx).
		Where("log_name = ?", logName).
		Order("created_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	err := query.Find(&activities).Error
	return activities, err
}

func (r *Repository) FindByBatch(ctx context.Context, batchUUID string) ([]ActivityLog, error) {
	var activities []ActivityLog
	err := r.db.WithContext(ctx).
		Where("batch_uuid = ?", batchUUID).
		Order("created_at ASC").
		Find(&activities).Error
	return activities, err
}

func (r *Repository) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&ActivityLog{}, "id = ?", id).Error
}

func (r *Repository) DeleteBySubject(ctx context.Context, subjectType, subjectID string) error {
	return r.db.WithContext(ctx).
		Where("subject_type = ? AND subject_id = ?", subjectType, subjectID).
		Delete(&ActivityLog{}).Error
}

func (r *Repository) CleanOlderThan(ctx context.Context, days int) (int64, error) {
	result := r.db.WithContext(ctx).
		Where("created_at < NOW() - INTERVAL ? DAY", days).
		Delete(&ActivityLog{})
	return result.RowsAffected, result.Error
}

type QueryBuilder struct {
	db *gorm.DB
}

func (r *Repository) Query() *QueryBuilder {
	return &QueryBuilder{db: r.db.Model(&ActivityLog{})}
}

func (q *QueryBuilder) WithSubject(subjectType, subjectID string) *QueryBuilder {
	q.db = q.db.Where("subject_type = ? AND subject_id = ?", subjectType, subjectID)
	return q
}

func (q *QueryBuilder) WithCauser(causerType, causerID string) *QueryBuilder {
	q.db = q.db.Where("causer_type = ? AND causer_id = ?", causerType, causerID)
	return q
}

func (q *QueryBuilder) WithLogName(logName string) *QueryBuilder {
	q.db = q.db.Where("log_name = ?", logName)
	return q
}

func (q *QueryBuilder) WithEvent(event string) *QueryBuilder {
	q.db = q.db.Where("event = ?", event)
	return q
}

func (q *QueryBuilder) Limit(limit int) *QueryBuilder {
	q.db = q.db.Limit(limit)
	return q
}

func (q *QueryBuilder) Offset(offset int) *QueryBuilder {
	q.db = q.db.Offset(offset)
	return q
}

func (q *QueryBuilder) OrderByLatest() *QueryBuilder {
	q.db = q.db.Order("created_at DESC")
	return q
}

func (q *QueryBuilder) OrderByOldest() *QueryBuilder {
	q.db = q.db.Order("created_at ASC")
	return q
}

func (q *QueryBuilder) Get(ctx context.Context) ([]ActivityLog, error) {
	var activities []ActivityLog
	err := q.db.WithContext(ctx).Find(&activities).Error
	return activities, err
}

func (q *QueryBuilder) First(ctx context.Context) (*ActivityLog, error) {
	var activity ActivityLog
	err := q.db.WithContext(ctx).First(&activity).Error
	if err != nil {
		return nil, err
	}
	return &activity, nil
}

func (q *QueryBuilder) Count(ctx context.Context) (int64, error) {
	var count int64
	err := q.db.WithContext(ctx).Count(&count).Error
	return count, err
}
