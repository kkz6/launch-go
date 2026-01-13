package repositories

import (
	"context"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/pkg/repository"
)

// BaseRepository provides common database operations.
// Deprecated: New repositories should embed repository.Base[T] directly.
// This is kept for backward compatibility during migration.
type BaseRepository struct {
	db *gorm.DB
}

// NewBaseRepository creates a new base repository
// Deprecated: Use repository.NewBase[T](db) instead.
func NewBaseRepository(db *gorm.DB) *BaseRepository {
	return &BaseRepository{db: db}
}

// DB returns the database connection
func (r *BaseRepository) DB() *gorm.DB {
	return r.db
}

// WithTransaction executes operations within a transaction
func (r *BaseRepository) WithTransaction(ctx context.Context, fn func(tx *gorm.DB) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(tx)
	})
}

// WrapNotFoundError wraps repository.ErrNotFound with a custom error.
// This helper allows repositories using generic Base[T] to return custom errors.
func WrapNotFoundError[T any](result *T, err error, customErr error) (*T, error) {
	if err != nil {
		if repository.IsNotFound(err) {
			return nil, customErr
		}
		return nil, err
	}
	return result, nil
}

// WrapNotFoundErrorSlice wraps repository.ErrNotFound for slice results.
func WrapNotFoundErrorSlice[T any](result []T, err error, customErr error) ([]T, error) {
	if err != nil {
		if repository.IsNotFound(err) {
			return nil, customErr
		}
		return nil, err
	}
	return result, nil
}
