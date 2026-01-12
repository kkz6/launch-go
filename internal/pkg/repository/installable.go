package repository

import (
	"context"
	"time"

	"gorm.io/gorm"
)

// Installable provides common operations for models with installation status.
// Embed this in your repository for models that use InstallableModel.
//
// Usage:
//
//	type CronRepository struct {
//	    repository.Installable[models.Cron]
//	}
type Installable[T any] struct {
	Base[T]
}

// NewInstallable creates a new Installable repository
func NewInstallable[T any](db *gorm.DB) Installable[T] {
	return Installable[T]{Base: NewBase[T](db)}
}

// MarkAsInstalled marks a record as installed
func (r *Installable[T]) MarkAsInstalled(ctx context.Context, id string) error {
	now := time.Now()
	return r.UpdateFields(ctx, id, map[string]interface{}{
		"installed_at":           now,
		"installation_failed_at": nil,
	})
}

// MarkAsFailed marks a record's installation as failed
func (r *Installable[T]) MarkAsFailed(ctx context.Context, id string) error {
	now := time.Now()
	return r.UpdateFields(ctx, id, map[string]interface{}{
		"installation_failed_at": now,
	})
}

// MarkAsUninstalling marks a record as being uninstalled
func (r *Installable[T]) MarkAsUninstalling(ctx context.Context, id string) error {
	now := time.Now()
	return r.UpdateFields(ctx, id, map[string]interface{}{
		"uninstallation_requested_at": now,
	})
}

// MarkUninstallationFailed marks a record's uninstallation as failed
func (r *Installable[T]) MarkUninstallationFailed(ctx context.Context, id string) error {
	now := time.Now()
	return r.UpdateFields(ctx, id, map[string]interface{}{
		"uninstallation_failed_at": now,
	})
}

// FindInstalled finds all installed records for a server
func (r *Installable[T]) FindInstalled(ctx context.Context, serverID string) ([]T, error) {
	var entities []T
	err := r.DB.WithContext(ctx).
		Where("server_id = ? AND installed_at IS NOT NULL AND installation_failed_at IS NULL", serverID).
		Order("created_at DESC").
		Find(&entities).Error
	return entities, err
}

// FindPending finds all pending (not yet installed) records for a server
func (r *Installable[T]) FindPending(ctx context.Context, serverID string) ([]T, error) {
	var entities []T
	err := r.DB.WithContext(ctx).
		Where("server_id = ? AND installed_at IS NULL AND installation_failed_at IS NULL", serverID).
		Order("created_at DESC").
		Find(&entities).Error
	return entities, err
}

// FindFailed finds all failed records for a server
func (r *Installable[T]) FindFailed(ctx context.Context, serverID string) ([]T, error) {
	var entities []T
	err := r.DB.WithContext(ctx).
		Where("server_id = ? AND installation_failed_at IS NOT NULL", serverID).
		Order("created_at DESC").
		Find(&entities).Error
	return entities, err
}

// FindUninstalling finds all records being uninstalled for a server
func (r *Installable[T]) FindUninstalling(ctx context.Context, serverID string) ([]T, error) {
	var entities []T
	err := r.DB.WithContext(ctx).
		Where("server_id = ? AND uninstallation_requested_at IS NOT NULL AND uninstallation_failed_at IS NULL", serverID).
		Order("created_at DESC").
		Find(&entities).Error
	return entities, err
}
