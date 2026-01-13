package jobs

import (
	"time"

	"gorm.io/gorm"
)

// InstallationTracker provides common installation tracking methods.
// Embed this in jobs that install things (services, crons, daemons, etc.)
//
// Usage:
//
//	type InstallCronJob struct {
//	    jobs.BaseJob
//	    jobs.InstallationTracker
//	    CronID string
//	}
//
//	func (j *InstallCronJob) Handle(ctx context.Context) error {
//	    // ... do installation work ...
//	    return j.MarkAsInstalled(j.DB, cron)
//	}
type InstallationTracker struct{}

// MarkAsInstalled marks a model as successfully installed
func (t *InstallationTracker) MarkAsInstalled(db *gorm.DB, model interface{}) error {
	now := time.Now()
	return db.Model(model).Updates(map[string]interface{}{
		"installed_at":           &now,
		"installation_failed_at": nil,
	}).Error
}

// MarkInstallationFailed marks a model's installation as failed
func (t *InstallationTracker) MarkInstallationFailed(db *gorm.DB, model interface{}) error {
	now := time.Now()
	return db.Model(model).Updates(map[string]interface{}{
		"installed_at":           nil,
		"installation_failed_at": &now,
	}).Error
}

// UninstallationTracker provides common uninstallation tracking methods.
// Embed this in jobs that uninstall things.
//
// Usage:
//
//	type UninstallCronJob struct {
//	    jobs.BaseJob
//	    jobs.UninstallationTracker
//	    CronID string
//	}
type UninstallationTracker struct{}

// MarkAsUninstalled marks a model as successfully uninstalled (deleted)
func (t *UninstallationTracker) MarkAsUninstalled(db *gorm.DB, model interface{}) error {
	return db.Delete(model).Error
}

// MarkUninstallationFailed marks a model's uninstallation as failed
func (t *UninstallationTracker) MarkUninstallationFailed(db *gorm.DB, model interface{}) error {
	now := time.Now()
	return db.Model(model).Updates(map[string]interface{}{
		"uninstallation_requested_at": nil,
		"uninstallation_failed_at":    &now,
	}).Error
}

// TaskTracker provides task tracking for jobs that create server tasks.
// Embed this in jobs that need to track task IDs.
type TaskTracker struct{}

// UpdateTaskID updates the task_id field on a model
func (t *TaskTracker) UpdateTaskID(db *gorm.DB, model interface{}, taskID string) error {
	return db.Model(model).Update("task_id", taskID).Error
}

// ClearTaskID clears the task_id field on a model
func (t *TaskTracker) ClearTaskID(db *gorm.DB, model interface{}) error {
	return db.Model(model).Update("task_id", nil).Error
}

// StatusTracker provides status update methods for jobs.
type StatusTracker struct{}

// UpdateStatus updates the status field on a model
func (t *StatusTracker) UpdateStatus(db *gorm.DB, model interface{}, status string) error {
	return db.Model(model).Update("status", status).Error
}

// UpdateFields updates multiple fields on a model
func (t *StatusTracker) UpdateFields(db *gorm.DB, model interface{}, fields map[string]interface{}) error {
	return db.Model(model).Updates(fields).Error
}
