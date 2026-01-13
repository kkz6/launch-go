package repositories

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/server/contracts"
	apperrors "github.com/kkz6/launch-go/internal/pkg/errors"
)

// Repository errors - re-exported from centralized error package
var (
	ErrServerNotFound         = apperrors.ErrServerNotFound
	ErrServiceNotFound        = apperrors.ErrServiceNotFound
	ErrFirewallRuleNotFound   = apperrors.ErrFirewallRuleNotFound
	ErrCronNotFound           = apperrors.ErrCronNotFound
	ErrDaemonNotFound         = apperrors.ErrDaemonNotFound
	ErrSshKeyNotFound         = apperrors.ErrSshKeyNotFound
	ErrTaskNotFound           = apperrors.ErrTaskNotFound
	ErrMetricNotFound         = apperrors.ErrMetricNotFound
	ErrServerProviderNotFound = apperrors.ErrServerProviderNotFound
	ErrDatabaseNotFound       = apperrors.ErrDatabaseNotFound
	ErrDatabaseUserNotFound   = apperrors.ErrDatabaseUserNotFound
)

// Repository provides database operations for server module
type Repository struct {
	db *gorm.DB
}

// NewRepository creates a new Repository instance
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// Transaction executes a function within a database transaction
func (r *Repository) Transaction(ctx context.Context, fn func(tx contracts.Repository) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(&Repository{db: tx})
	})
}

// DB returns the underlying database connection
func (r *Repository) DB() *gorm.DB {
	return r.db
}

// Generic helper methods for common operations

// create is a generic helper for creating entities
func create[T any](r *Repository, ctx context.Context, entity *T) error {
	return r.db.WithContext(ctx).Create(entity).Error
}

// findByID is a generic helper for finding by ID
func findByID[T any](r *Repository, ctx context.Context, id string, notFoundErr error) (*T, error) {
	var entity T
	err := r.db.WithContext(ctx).First(&entity, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, notFoundErr
		}
		return nil, err
	}
	return &entity, nil
}

// findByIDWithPreload is a generic helper for finding by ID with preloading
func findByIDWithPreload[T any](r *Repository, ctx context.Context, id string, preload string, notFoundErr error) (*T, error) {
	var entity T
	err := r.db.WithContext(ctx).Preload(preload).First(&entity, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, notFoundErr
		}
		return nil, err
	}
	return &entity, nil
}

// findByIDAndServer is a generic helper for finding by ID and server ID
func findByIDAndServer[T any](r *Repository, ctx context.Context, id, serverID string, notFoundErr error) (*T, error) {
	var entity T
	err := r.db.WithContext(ctx).First(&entity, "id = ? AND server_id = ?", id, serverID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, notFoundErr
		}
		return nil, err
	}
	return &entity, nil
}

// findByServer is a generic helper for finding all by server ID
func findByServer[T any](r *Repository, ctx context.Context, serverID string) ([]T, error) {
	var entities []T
	err := r.db.WithContext(ctx).
		Where("server_id = ?", serverID).
		Order("created_at DESC").
		Find(&entities).Error
	return entities, err
}

// update is a generic helper for updating entities
func update[T any](r *Repository, ctx context.Context, entity *T) error {
	return r.db.WithContext(ctx).Save(entity).Error
}

// deleteByID is a generic helper for deleting by ID
func deleteByID[T any](r *Repository, ctx context.Context, id string) error {
	var entity T
	return r.db.WithContext(ctx).Delete(&entity, "id = ?", id).Error
}

// markAsInstalled is a generic helper for marking entities as installed
func markAsInstalled[T any](r *Repository, ctx context.Context, id string) error {
	var entity T
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&entity).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"installed_at":           now,
			"installation_failed_at": nil,
		}).Error
}

// markAsFailed is a generic helper for marking installation as failed
func markAsFailed[T any](r *Repository, ctx context.Context, id string) error {
	var entity T
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&entity).
		Where("id = ?", id).
		Update("installation_failed_at", now).Error
}
