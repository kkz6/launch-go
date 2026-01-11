package repositories

import (
	"context"

	"github.com/kkz6/launch-go/internal/modules/server/models"
)

// FindDatabasesByServer returns all databases for a server
func (r *Repository) FindDatabasesByServer(ctx context.Context, serverID string) ([]models.Database, error) {
	var databases []models.Database
	err := r.db.WithContext(ctx).
		Where("server_id = ?", serverID).
		Order("created_at DESC").
		Find(&databases).Error
	return databases, err
}

// FindDatabaseByID returns a database by ID
func (r *Repository) FindDatabaseByID(ctx context.Context, id string) (*models.Database, error) {
	var database models.Database
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&database).Error
	if err != nil {
		return nil, err
	}
	return &database, nil
}

// FindDatabaseByIDWithServer returns a database by ID with the Server relation preloaded
func (r *Repository) FindDatabaseByIDWithServer(ctx context.Context, id string) (*models.Database, error) {
	var database models.Database
	err := r.db.WithContext(ctx).Preload("Server").Where("id = ?", id).First(&database).Error
	if err != nil {
		return nil, err
	}
	return &database, nil
}

// CreateDatabase creates a new database
func (r *Repository) CreateDatabase(ctx context.Context, db *models.Database) error {
	return r.db.WithContext(ctx).Create(db).Error
}

// DeleteDatabase deletes a database
func (r *Repository) DeleteDatabase(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&models.Database{}, "id = ?", id).Error
}

// FindDatabaseUsersByServer returns all database users for a server
func (r *Repository) FindDatabaseUsersByServer(ctx context.Context, serverID string) ([]models.DatabaseUser, error) {
	var users []models.DatabaseUser
	err := r.db.WithContext(ctx).
		Where("server_id = ?", serverID).
		Order("created_at DESC").
		Find(&users).Error
	return users, err
}

// FindDatabaseUserByID returns a database user by ID
func (r *Repository) FindDatabaseUserByID(ctx context.Context, id string) (*models.DatabaseUser, error) {
	var user models.DatabaseUser
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// FindDatabaseUserByIDWithServer returns a database user by ID with the Server relation preloaded
func (r *Repository) FindDatabaseUserByIDWithServer(ctx context.Context, id string) (*models.DatabaseUser, error) {
	var user models.DatabaseUser
	err := r.db.WithContext(ctx).Preload("Server").Preload("Databases").Where("id = ?", id).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// CreateDatabaseUser creates a new database user
func (r *Repository) CreateDatabaseUser(ctx context.Context, user *models.DatabaseUser) error {
	return r.db.WithContext(ctx).Create(user).Error
}

// DeleteDatabaseUser deletes a database user
func (r *Repository) DeleteDatabaseUser(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&models.DatabaseUser{}, "id = ?", id).Error
}
