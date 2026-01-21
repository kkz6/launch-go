package repositories

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/database/models"
	"github.com/kkz6/launch-go/internal/pkg/repository"
)

// DatabaseRepository handles database-related database operations for the server module
type DatabaseRepository struct {
	repository.Base[models.Database]
}

// NewDatabaseRepository creates a new DatabaseRepository instance
func NewDatabaseRepository(db *gorm.DB) *DatabaseRepository {
	return &DatabaseRepository{
		Base: repository.NewBase[models.Database](db),
	}
}

// FindByID returns a database by ID
func (r *DatabaseRepository) FindByID(ctx context.Context, id string) (*models.Database, error) {
	database, err := r.Base.FindByID(ctx, id)
	if err != nil {
		if repository.IsNotFound(err) {
			return nil, ErrDatabaseNotFound
		}
		return nil, err
	}
	return database, nil
}

// FindByIDWithServer returns a database by ID with the Server relation preloaded
func (r *DatabaseRepository) FindByIDWithServer(ctx context.Context, id string) (*models.Database, error) {
	var database models.Database
	err := r.DB.WithContext(ctx).Preload("Server").First(&database, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDatabaseNotFound
		}
		return nil, err
	}
	return &database, nil
}

// FindUsersByServer returns all database users for a server
func (r *DatabaseRepository) FindUsersByServer(ctx context.Context, serverID string) ([]models.DatabaseUser, error) {
	var users []models.DatabaseUser
	err := r.DB.WithContext(ctx).
		Where("server_id = ?", serverID).
		Order("created_at DESC").
		Find(&users).Error
	return users, err
}

// FindUserByID returns a database user by ID
func (r *DatabaseRepository) FindUserByID(ctx context.Context, id string) (*models.DatabaseUser, error) {
	var user models.DatabaseUser
	err := r.DB.WithContext(ctx).First(&user, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDatabaseUserNotFound
		}
		return nil, err
	}
	return &user, nil
}

// FindUserByIDWithServer returns a database user by ID with the Server relation preloaded
func (r *DatabaseRepository) FindUserByIDWithServer(ctx context.Context, id string) (*models.DatabaseUser, error) {
	var user models.DatabaseUser
	err := r.DB.WithContext(ctx).Preload("Server").Preload("Databases").Where("id = ?", id).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDatabaseUserNotFound
		}
		return nil, err
	}
	return &user, nil
}

// CreateUser creates a new database user
func (r *DatabaseRepository) CreateUser(ctx context.Context, user *models.DatabaseUser) error {
	return r.DB.WithContext(ctx).Create(user).Error
}

// DeleteUser deletes a database user
func (r *DatabaseRepository) DeleteUser(ctx context.Context, id string) error {
	return r.DB.WithContext(ctx).Delete(&models.DatabaseUser{}, "id = ?", id).Error
}
