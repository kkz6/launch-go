package repositories

import (
	"context"

	"github.com/kkz6/launch-go/internal/modules/database/models"
)

// FindByDatabase finds all database users for a database
func (r *DatabaseUserRepository) FindByDatabase(ctx context.Context, databaseID string) ([]models.DatabaseUser, error) {
	var users []models.DatabaseUser

	err := r.DB.WithContext(ctx).
		Joins("JOIN database_database_user ON database_database_user.database_user_id = database_users.id").
		Where("database_database_user.database_id = ?", databaseID).
		Find(&users).Error

	return users, err
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
