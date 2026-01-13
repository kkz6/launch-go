package repositories

import (
	"context"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/database/models"
	apperrors "github.com/kkz6/launch-go/internal/pkg/errors"
	"github.com/kkz6/launch-go/internal/pkg/repository"
)

// Repository errors - re-exported from centralized error package
var (
	ErrDatabaseNotFound     = apperrors.ErrDatabaseNotFound
	ErrDatabaseUserNotFound = apperrors.ErrDatabaseUserNotFound
	ErrDuplicateName        = apperrors.Conflict("A database with this name already exists on this server")
	ErrDuplicateUserName    = apperrors.Conflict("A database user with this name already exists on this server")
)

// Repository provides database operations for database module
type Repository struct {
	db       *gorm.DB
	database repository.Installable[models.Database]
	user     repository.Installable[models.DatabaseUser]
}

// NewRepository creates a new Repository instance
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{
		db:       db,
		database: repository.NewInstallable[models.Database](db),
		user:     repository.NewInstallable[models.DatabaseUser](db),
	}
}

// Transaction executes a function within a database transaction
func (r *Repository) Transaction(ctx context.Context, fn func(tx *Repository) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(NewRepository(tx))
	})
}

// DB returns the underlying database connection
func (r *Repository) DB() *gorm.DB {
	return r.db
}

// DatabaseRepo returns the database repository with generic CRUD operations
func (r *Repository) DatabaseRepo() *repository.Installable[models.Database] {
	return &r.database
}

// UserRepo returns the database user repository with generic CRUD operations
func (r *Repository) UserRepo() *repository.Installable[models.DatabaseUser] {
	return &r.user
}
