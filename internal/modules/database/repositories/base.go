package repositories

import (
	"context"
	"errors"

	"gorm.io/gorm"
)

var (
	ErrDatabaseNotFound     = errors.New("database not found")
	ErrDatabaseUserNotFound = errors.New("database user not found")
	ErrDuplicateName        = errors.New("a database with this name already exists on this server")
	ErrDuplicateUserName    = errors.New("a database user with this name already exists on this server")
)

// Repository provides database operations for database module
type Repository struct {
	db *gorm.DB
}

// NewRepository creates a new Repository instance
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// Transaction executes a function within a database transaction
func (r *Repository) Transaction(ctx context.Context, fn func(tx *Repository) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(&Repository{db: tx})
	})
}
