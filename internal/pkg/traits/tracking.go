// Package traits provides embeddable structs that add behavior to models and jobs.
// Similar to Laravel traits, these can be embedded to gain common functionality.
package traits

import (
	"time"

	"gorm.io/gorm"
)

// Installable is implemented by models that can track installation status.
type Installable interface {
	GetID() string
	TableName() string
}

// Uninstallable is implemented by models that can be uninstalled/deleted.
type Uninstallable interface {
	GetID() string
	TableName() string
}

// StatusTrackable is implemented by models that have a status field.
type StatusTrackable interface {
	GetID() string
	TableName() string
}

// TaskTrackable is implemented by models that track server task IDs.
type TaskTrackable interface {
	GetID() string
	TableName() string
}

// InstallationTracker provides common installation tracking methods.
// Embed this in jobs that install things (services, crons, daemons, etc.)
//
// Usage:
//
//	type InstallCronJob struct {
//	    *JobContext
//	    traits.InstallationTracker
//	    CronID string
//	}
//
//	func (j *InstallCronJob) Handle(ctx context.Context) error {
//	    // ... do installation work ...
//	    return j.MarkAsInstalled(j.DB, cron)
//	}
type InstallationTracker struct{}

// MarkAsInstalled marks a model as successfully installed
func (t *InstallationTracker) MarkAsInstalled(db *gorm.DB, model any) error {
	now := time.Now()
	return db.Model(model).Updates(map[string]any{
		"installed_at":           &now,
		"installation_failed_at": nil,
	}).Error
}

// MarkInstallationFailed marks a model's installation as failed
func (t *InstallationTracker) MarkInstallationFailed(db *gorm.DB, model any) error {
	now := time.Now()
	return db.Model(model).Updates(map[string]any{
		"installed_at":           nil,
		"installation_failed_at": &now,
	}).Error
}

// UninstallationTracker provides common uninstallation tracking methods.
// Embed this in jobs that uninstall things.
type UninstallationTracker struct{}

// MarkAsUninstalled marks a model as successfully uninstalled (deleted)
func (t *UninstallationTracker) MarkAsUninstalled(db *gorm.DB, model any) error {
	return db.Delete(model).Error
}

// MarkUninstallationFailed marks a model's uninstallation as failed
func (t *UninstallationTracker) MarkUninstallationFailed(db *gorm.DB, model any) error {
	now := time.Now()
	return db.Model(model).Updates(map[string]any{
		"uninstallation_requested_at": nil,
		"uninstallation_failed_at":    &now,
	}).Error
}

// TaskTracker provides task tracking for jobs that create server tasks.
// Embed this in jobs that need to track task IDs.
type TaskTracker struct{}

// UpdateTaskID updates the task_id field on a model
func (t *TaskTracker) UpdateTaskID(db *gorm.DB, model any, taskID string) error {
	return db.Model(model).Update("task_id", taskID).Error
}

// ClearTaskID clears the task_id field on a model
func (t *TaskTracker) ClearTaskID(db *gorm.DB, model any) error {
	return db.Model(model).Update("task_id", nil).Error
}

// StatusTracker provides status update methods for jobs.
type StatusTracker struct{}

// UpdateStatus updates the status field on a model
func (t *StatusTracker) UpdateStatus(db *gorm.DB, model any, status string) error {
	return db.Model(model).Update("status", status).Error
}

// UpdateFields updates multiple fields on a model
func (t *StatusTracker) UpdateFields(db *gorm.DB, model any, fields map[string]any) error {
	return db.Model(model).Updates(fields).Error
}

// TypedInstallationTracker provides type-safe installation tracking.
// Use this when you want compile-time guarantees that the model
// implements the Installable interface.
type TypedInstallationTracker[T Installable] struct {
	db *gorm.DB
}

// SetTrackerDB sets the database connection for the tracker.
func (t *TypedInstallationTracker[T]) SetTrackerDB(db *gorm.DB) {
	t.db = db
}

// MarkInstalled marks a model as successfully installed.
func (t *TypedInstallationTracker[T]) MarkInstalled(model T) error {
	now := time.Now()
	return t.db.Table(model.TableName()).
		Where("id = ?", model.GetID()).
		Updates(map[string]any{
			"installed_at":           &now,
			"installation_failed_at": nil,
		}).Error
}

// MarkInstallFailed marks a model's installation as failed.
func (t *TypedInstallationTracker[T]) MarkInstallFailed(model T) error {
	now := time.Now()
	return t.db.Table(model.TableName()).
		Where("id = ?", model.GetID()).
		Updates(map[string]any{
			"installed_at":           nil,
			"installation_failed_at": &now,
		}).Error
}

// TypedUninstallationTracker provides type-safe uninstallation tracking.
type TypedUninstallationTracker[T Uninstallable] struct {
	db *gorm.DB
}

// SetTrackerDB sets the database connection for the tracker.
func (t *TypedUninstallationTracker[T]) SetTrackerDB(db *gorm.DB) {
	t.db = db
}

// MarkUninstalled marks a model as successfully uninstalled (deleted).
func (t *TypedUninstallationTracker[T]) MarkUninstalled(model T) error {
	return t.db.Table(model.TableName()).
		Where("id = ?", model.GetID()).
		Delete(model).Error
}

// MarkUninstallFailed marks a model's uninstallation as failed.
func (t *TypedUninstallationTracker[T]) MarkUninstallFailed(model T) error {
	now := time.Now()
	return t.db.Table(model.TableName()).
		Where("id = ?", model.GetID()).
		Updates(map[string]any{
			"uninstallation_requested_at": nil,
			"uninstallation_failed_at":    &now,
		}).Error
}

// TypedStatusTracker provides type-safe status tracking.
type TypedStatusTracker[T StatusTrackable] struct {
	db *gorm.DB
}

// SetTrackerDB sets the database connection for the tracker.
func (t *TypedStatusTracker[T]) SetTrackerDB(db *gorm.DB) {
	t.db = db
}

// SetStatus updates the status field on a model.
func (t *TypedStatusTracker[T]) SetStatus(model T, status string) error {
	return t.db.Table(model.TableName()).
		Where("id = ?", model.GetID()).
		Update("status", status).Error
}

// SetFields updates multiple fields on a model.
func (t *TypedStatusTracker[T]) SetFields(model T, fields map[string]any) error {
	return t.db.Table(model.TableName()).
		Where("id = ?", model.GetID()).
		Updates(fields).Error
}

// TypedTaskTracker provides type-safe task ID tracking.
type TypedTaskTracker[T TaskTrackable] struct {
	db *gorm.DB
}

// SetTrackerDB sets the database connection for the tracker.
func (t *TypedTaskTracker[T]) SetTrackerDB(db *gorm.DB) {
	t.db = db
}

// SetTaskID updates the task_id field on a model.
func (t *TypedTaskTracker[T]) SetTaskID(model T, taskID string) error {
	return t.db.Table(model.TableName()).
		Where("id = ?", model.GetID()).
		Update("task_id", taskID).Error
}

// ClearTask clears the task_id field on a model.
func (t *TypedTaskTracker[T]) ClearTask(model T) error {
	return t.db.Table(model.TableName()).
		Where("id = ?", model.GetID()).
		Update("task_id", nil).Error
}
