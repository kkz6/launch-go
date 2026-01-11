package jobs

import (
	"time"

	"gorm.io/gorm"
)

// Installable interface for models that can be installed
type Installable interface {
	SetInstalledAt(t *time.Time)
	SetInstallationFailedAt(t *time.Time)
}

// Uninstallable interface for models that can be uninstalled
type Uninstallable interface {
	SetUninstalledAt(t *time.Time)
	SetUninstallationFailedAt(t *time.Time)
}

// InstallationTracker provides common installation tracking methods
// Embed this in jobs that install things (services, crons, daemons, etc.)
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

// UninstallationTracker provides common uninstallation tracking methods
// Embed this in jobs that uninstall things
type UninstallationTracker struct{}

// MarkAsUninstalled marks a model as successfully uninstalled
func (t *UninstallationTracker) MarkAsUninstalled(db *gorm.DB, model interface{}) error {
	now := time.Now()
	return db.Model(model).Updates(map[string]interface{}{
		"uninstalled_at":           &now,
		"uninstallation_failed_at": nil,
	}).Error
}

// MarkUninstallationFailed marks a model's uninstallation as failed
func (t *UninstallationTracker) MarkUninstallationFailed(db *gorm.DB, model interface{}) error {
	now := time.Now()
	return db.Model(model).Updates(map[string]interface{}{
		"uninstalled_at":           nil,
		"uninstallation_failed_at": &now,
	}).Error
}

// ServiceTracker provides tracking for services with task_id
type ServiceTracker struct {
	InstallationTracker
}

// UpdateTaskID updates the task_id field on a service model
func (t *ServiceTracker) UpdateTaskID(db *gorm.DB, model interface{}, taskID string) error {
	return db.Model(model).Update("task_id", taskID).Error
}
