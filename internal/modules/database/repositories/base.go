package repositories

import (
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/database/contracts"
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

// Ensure Registry implements contracts.RepositoryRegistry
var _ contracts.RepositoryRegistry = (*Registry)(nil)

// Registry provides access to all database module repositories
type Registry struct {
	db       *gorm.DB
	database *DatabaseRepository
	user     *DatabaseUserRepository
}

// NewRegistry creates a new Registry instance
func NewRegistry(db *gorm.DB) *Registry {
	return &Registry{
		db:       db,
		database: NewDatabaseRepository(db),
		user:     NewDatabaseUserRepository(db),
	}
}

// Database returns the database repository
func (r *Registry) Database() contracts.DatabaseRepository {
	return r.database
}

// User returns the database user repository
func (r *Registry) User() contracts.DatabaseUserRepository {
	return r.user
}

// DB returns the underlying database connection
func (r *Registry) DB() *gorm.DB {
	return r.db
}

// DatabaseRepository provides database operations.
type DatabaseRepository struct {
	repository.Installable[models.Database]
}

// NewDatabaseRepository creates a new DatabaseRepository instance
func NewDatabaseRepository(db *gorm.DB) *DatabaseRepository {
	return &DatabaseRepository{
		Installable: repository.NewInstallable[models.Database](db),
	}
}

// DatabaseUserRepository provides database user operations.
type DatabaseUserRepository struct {
	repository.Installable[models.DatabaseUser]
}

// NewDatabaseUserRepository creates a new DatabaseUserRepository instance
func NewDatabaseUserRepository(db *gorm.DB) *DatabaseUserRepository {
	return &DatabaseUserRepository{
		Installable: repository.NewInstallable[models.DatabaseUser](db),
	}
}
