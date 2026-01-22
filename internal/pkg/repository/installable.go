package repository

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

// Installable provides common operations for models with installation status.
// Embed this in your repository for models that use InstallableModel.
//
// These models typically have:
//   - server_id: foreign key to servers table
//   - installed_at: timestamp when installed
//   - installation_failed_at: timestamp when installation failed
//   - uninstallation_requested_at: timestamp when uninstall was requested
//   - uninstallation_failed_at: timestamp when uninstallation failed
//
// Usage:
//
//	type CronRepository struct {
//	    repository.Installable[models.Cron]
//	}
//
//	func NewCronRepository(db *gorm.DB) *CronRepository {
//	    return &CronRepository{
//	        Installable: repository.NewInstallable[models.Cron](db),
//	    }
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

// MarkInstallationFailed marks a record's installation as failed and clears installed_at.
// Use this when an installation fails to ensure both the failure timestamp is set
// and the installed_at is cleared (in case of partial installation).
func (r *Installable[T]) MarkInstallationFailed(ctx context.Context, id string) error {
	now := time.Now()
	return r.UpdateFields(ctx, id, map[string]interface{}{
		"installed_at":           nil,
		"installation_failed_at": &now,
	})
}

// MarkAsUninstalling marks a record as being uninstalled
func (r *Installable[T]) MarkAsUninstalling(ctx context.Context, id string) error {
	now := time.Now()
	return r.UpdateFields(ctx, id, map[string]interface{}{
		"uninstallation_requested_at": now,
	})
}

// MarkUninstallationFailed marks a record's uninstallation as failed and clears uninstallation_requested_at.
// Use this when an uninstallation fails to reset the state for potential retry.
func (r *Installable[T]) MarkUninstallationFailed(ctx context.Context, id string) error {
	now := time.Now()
	return r.UpdateFields(ctx, id, map[string]interface{}{
		"uninstallation_requested_at": nil,
		"uninstallation_failed_at":    &now,
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

// FindByIDWithServer finds a record by ID with the Server relation preloaded
func (r *Installable[T]) FindByIDWithServer(ctx context.Context, id string) (*T, error) {
	var entity T
	err := r.DB.WithContext(ctx).Preload("Server").First(&entity, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &entity, nil
}

// FindByIDWithServerOrFail finds a record by ID with the Server relation preloaded or returns a typed error
func (r *Installable[T]) FindByIDWithServerOrFail(ctx context.Context, id string) (*T, error) {
	entity, err := r.FindByIDWithServer(ctx, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, NotFoundError(r.getModelName(), id)
		}
		return nil, WrapError(err, r.getModelName(), "failed to find "+r.getModelName())
	}
	return entity, nil
}

// DeleteByServer deletes a record by ID and server ID (safer than Delete)
func (r *Installable[T]) DeleteByServer(ctx context.Context, id, serverID string) error {
	var entity T
	result := r.DB.WithContext(ctx).
		Where("id = ? AND server_id = ?", id, serverID).
		Delete(&entity)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// FindVisibleByServer finds all visible (non-hidden) records for a server
// This is useful for models that have a "hidden" field for system-managed entries
func (r *Installable[T]) FindVisibleByServer(ctx context.Context, serverID string) ([]T, error) {
	var entities []T
	err := r.DB.WithContext(ctx).
		Where("server_id = ? AND hidden = ?", serverID, false).
		Order("created_at DESC").
		Find(&entities).Error
	return entities, err
}

// FindByServerPaginated finds all records for a server with pagination
func (r *Installable[T]) FindByServerPaginated(ctx context.Context, serverID string, page, perPage int) ([]T, int64, error) {
	var entities []T
	var total int64

	// Get total count
	var entity T
	if err := r.DB.WithContext(ctx).
		Model(&entity).
		Where("server_id = ?", serverID).
		Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Get paginated results
	offset := (page - 1) * perPage
	err := r.DB.WithContext(ctx).
		Where("server_id = ?", serverID).
		Order("created_at DESC").
		Offset(offset).
		Limit(perPage).
		Find(&entities).Error

	return entities, total, err
}

// CountInstalled returns the count of installed records for a server
func (r *Installable[T]) CountInstalled(ctx context.Context, serverID string) (int64, error) {
	var count int64
	var entity T
	err := r.DB.WithContext(ctx).
		Model(&entity).
		Where("server_id = ? AND installed_at IS NOT NULL AND installation_failed_at IS NULL", serverID).
		Count(&count).Error
	return count, err
}

// CountPending returns the count of pending (not yet installed) records for a server
func (r *Installable[T]) CountPending(ctx context.Context, serverID string) (int64, error) {
	var count int64
	var entity T
	err := r.DB.WithContext(ctx).
		Model(&entity).
		Where("server_id = ? AND installed_at IS NULL AND installation_failed_at IS NULL", serverID).
		Count(&count).Error
	return count, err
}

// CountFailed returns the count of failed records for a server
func (r *Installable[T]) CountFailed(ctx context.Context, serverID string) (int64, error) {
	var count int64
	var entity T
	err := r.DB.WithContext(ctx).
		Model(&entity).
		Where("server_id = ? AND installation_failed_at IS NOT NULL", serverID).
		Count(&count).Error
	return count, err
}

// IsInstalled checks if a record is installed
func (r *Installable[T]) IsInstalled(ctx context.Context, id string) (bool, error) {
	var count int64
	var entity T
	err := r.DB.WithContext(ctx).
		Model(&entity).
		Where("id = ? AND installed_at IS NOT NULL AND installation_failed_at IS NULL", id).
		Count(&count).Error
	return count > 0, err
}

// ClearInstallationFailure clears the installation_failed_at timestamp for retry
func (r *Installable[T]) ClearInstallationFailure(ctx context.Context, id string) error {
	return r.UpdateFields(ctx, id, map[string]interface{}{
		"installation_failed_at": nil,
	})
}

// ClearUninstallationFailure clears the uninstallation_failed_at timestamp for retry
func (r *Installable[T]) ClearUninstallationFailure(ctx context.Context, id string) error {
	return r.UpdateFields(ctx, id, map[string]interface{}{
		"uninstallation_failed_at": nil,
	})
}
