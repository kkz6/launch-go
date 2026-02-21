package repositories

import (
	"context"
	"errors"

	"gorm.io/gorm"

	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"

	"github.com/kkz6/launch-go/internal/modules/database/models"
)

// FindByIDsAndServer finds databases by a list of IDs belonging to the given server
func (r *DatabaseRepository) FindByIDsAndServer(ctx context.Context, ids []string, serverID string) ([]models.Database, error) {
	var databases []models.Database

	err := r.DB.WithContext(ctx).
		Where("id IN ? AND server_id = ?", ids, serverID).
		Find(&databases).Error

	return databases, err
}

// FindByIDAndServerAndTeam finds a database by ID, server ID, and team ID
func (r *DatabaseRepository) FindByIDAndServerAndTeam(ctx context.Context, id, serverID, teamID string) (*models.Database, error) {
	var database models.Database

	err := r.QueryCtx(ctx).
		First(&database, "id = ? AND server_id = ? AND team_id = ?", id, serverID, teamID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fiberutil.NotFound()
		}

		return nil, err
	}

	return &database, nil
}

// CountByTeam counts all databases for a team
func (r *DatabaseRepository) CountByTeam(ctx context.Context, teamID string) (int64, error) {
	var count int64
	err := r.DB.WithContext(ctx).
		Model(&models.Database{}).
		Where("team_id = ?", teamID).
		Count(&count).Error

	return count, err
}

// FindByUser finds all databases for a database user
func (r *DatabaseRepository) FindByUser(ctx context.Context, userID string) ([]models.Database, error) {
	var databases []models.Database

	err := r.DB.WithContext(ctx).
		Joins("JOIN database_database_user ON database_database_user.database_id = databases.id").
		Where("database_database_user.database_user_id = ?", userID).
		Find(&databases).Error

	return databases, err
}

// AttachUser attaches a database user to a database
func (r *DatabaseRepository) AttachUser(ctx context.Context, databaseID, userID string) error {
	return r.DB.WithContext(ctx).
		Create(&models.DatabaseDatabaseUser{
			DatabaseID:     databaseID,
			DatabaseUserID: userID,
		}).Error
}

// DetachUser detaches a database user from a database
func (r *DatabaseRepository) DetachUser(ctx context.Context, databaseID, userID string) error {
	return r.DB.WithContext(ctx).
		Where("database_id = ? AND database_user_id = ?", databaseID, userID).
		Delete(&models.DatabaseDatabaseUser{}).Error
}

// DetachAllUsers detaches all users from a database
func (r *DatabaseRepository) DetachAllUsers(ctx context.Context, databaseID string) error {
	return r.DB.WithContext(ctx).
		Where("database_id = ?", databaseID).
		Delete(&models.DatabaseDatabaseUser{}).Error
}
