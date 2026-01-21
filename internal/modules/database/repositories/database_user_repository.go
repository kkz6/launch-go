package repositories

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/database/models"
)

// Create creates a new database user
func (r *DatabaseUserRepository) Create(ctx context.Context, user *models.DatabaseUser) error {
	return r.Installable.Create(ctx, user)
}

// FindByID finds a database user by ID
func (r *DatabaseUserRepository) FindByID(ctx context.Context, id string) (*models.DatabaseUser, error) {
	var user models.DatabaseUser

	err := r.DB.WithContext(ctx).
		Preload("Databases").
		First(&user, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDatabaseUserNotFound
		}

		return nil, err
	}

	return &user, nil
}

// FindByIDAndServer finds a database user by ID and server ID
func (r *DatabaseUserRepository) FindByIDAndServer(ctx context.Context, id, serverID string) (*models.DatabaseUser, error) {
	var user models.DatabaseUser

	err := r.DB.WithContext(ctx).
		Preload("Databases").
		First(&user, "id = ? AND server_id = ?", id, serverID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDatabaseUserNotFound
		}

		return nil, err
	}

	return &user, nil
}

// FindByServer finds all database users for a server
func (r *DatabaseUserRepository) FindByServer(ctx context.Context, serverID string) ([]models.DatabaseUser, error) {
	var users []models.DatabaseUser

	err := r.DB.WithContext(ctx).
		Preload("Databases").
		Where("server_id = ?", serverID).
		Order("created_at DESC").
		Find(&users).Error

	return users, err
}

// FindByNameAndServer finds a database user by name and server ID
func (r *DatabaseUserRepository) FindByNameAndServer(ctx context.Context, name, serverID string) (*models.DatabaseUser, error) {
	var user models.DatabaseUser

	err := r.DB.WithContext(ctx).
		Preload("Databases").
		First(&user, "name = ? AND server_id = ?", name, serverID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDatabaseUserNotFound
		}

		return nil, err
	}

	return &user, nil
}

// FindByDatabase finds all database users for a database
func (r *DatabaseUserRepository) FindByDatabase(ctx context.Context, databaseID string) ([]models.DatabaseUser, error) {
	var users []models.DatabaseUser

	err := r.DB.WithContext(ctx).
		Joins("JOIN database_database_user ON database_database_user.database_user_id = database_users.id").
		Where("database_database_user.database_id = ?", databaseID).
		Find(&users).Error

	return users, err
}

// Update updates a database user
func (r *DatabaseUserRepository) Update(ctx context.Context, user *models.DatabaseUser) error {
	return r.Installable.Update(ctx, user)
}

// UpdateFields updates specific fields of a database user
func (r *DatabaseUserRepository) UpdateFields(ctx context.Context, id string, fields map[string]interface{}) error {
	return r.Installable.UpdateFields(ctx, id, fields)
}

// Delete deletes a database user
func (r *DatabaseUserRepository) Delete(ctx context.Context, id string) error {
	return r.Installable.Delete(ctx, id)
}

// ExistsByNameAndServer checks if a database user exists with the given name on the server
func (r *DatabaseUserRepository) ExistsByNameAndServer(ctx context.Context, name, serverID string) (bool, error) {
	return r.Installable.ExistsByNameAndServer(ctx, name, serverID)
}

// MarkAsInstalled marks a database user as installed
func (r *DatabaseUserRepository) MarkAsInstalled(ctx context.Context, id string) error {
	return r.Installable.MarkAsInstalled(ctx, id)
}

// MarkAsFailed marks a database user installation as failed
func (r *DatabaseUserRepository) MarkAsFailed(ctx context.Context, id string) error {
	return r.Installable.MarkAsFailed(ctx, id)
}

// MarkAsUninstalling marks a database user as being uninstalled
func (r *DatabaseUserRepository) MarkAsUninstalling(ctx context.Context, id string) error {
	return r.Installable.MarkAsUninstalling(ctx, id)
}

// SyncDatabases syncs the databases attached to a user
func (r *DatabaseUserRepository) SyncDatabases(ctx context.Context, userID string, databaseIDs []string) error {
	// Delete existing associations
	if err := r.DB.WithContext(ctx).
		Where("database_user_id = ?", userID).
		Delete(&models.DatabaseDatabaseUser{}).Error; err != nil {
		return err
	}

	// Create new associations
	for _, dbID := range databaseIDs {
		if err := r.DB.WithContext(ctx).
			Create(&models.DatabaseDatabaseUser{
				DatabaseID:     dbID,
				DatabaseUserID: userID,
			}).Error; err != nil {
			return err
		}
	}

	return nil
}

// FindRootUser finds the root user for a server
func (r *DatabaseUserRepository) FindRootUser(ctx context.Context, serverID string) (*models.DatabaseUser, error) {
	return r.FindByNameAndServer(ctx, "root", serverID)
}
