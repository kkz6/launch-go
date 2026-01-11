package repositories

import (
	"context"

	"gorm.io/gorm"
)

// BaseRepository provides common database functionality
type BaseRepository struct {
	db *gorm.DB
}

// NewBaseRepository creates a new BaseRepository instance
func NewBaseRepository(db *gorm.DB) *BaseRepository {
	return &BaseRepository{db: db}
}

// DB returns the underlying database connection
func (r *BaseRepository) DB() *gorm.DB {
	return r.db
}

// BeginTransaction begins a new transaction
func (r *BaseRepository) BeginTransaction(ctx context.Context) *gorm.DB {
	return r.db.WithContext(ctx).Begin()
}

// WithTransaction executes a function within a transaction
func (r *BaseRepository) WithTransaction(ctx context.Context, fn func(tx *gorm.DB) error) error {
	return r.db.WithContext(ctx).Transaction(fn)
}
