package database

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

// Repository handles database operations for Database and DatabaseUser entities
type Repository struct {
	db *gorm.DB
}

// NewRepository creates a new database repository
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// Database operations

// Create creates a new database
func (r *Repository) Create(ctx context.Context, database *Database) error {
	return r.db.WithContext(ctx).Create(database).Error
}

// FindByID finds a database by ID
func (r *Repository) FindByID(ctx context.Context, id string) (*Database, error) {
	var database Database

	err := r.db.WithContext(ctx).
		Preload("Users").
		First(&database, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDatabaseNotFound
		}

		return nil, err
	}

	return &database, nil
}

// FindByIDAndServer finds a database by ID and server ID
func (r *Repository) FindByIDAndServer(ctx context.Context, id, serverID string) (*Database, error) {
	var database Database

	err := r.db.WithContext(ctx).
		Preload("Users").
		First(&database, "id = ? AND server_id = ?", id, serverID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDatabaseNotFound
		}

		return nil, err
	}

	return &database, nil
}

// FindByServer finds all databases for a server
func (r *Repository) FindByServer(ctx context.Context, serverID string) ([]Database, error) {
	var databases []Database

	err := r.db.WithContext(ctx).
		Preload("Users").
		Where("server_id = ?", serverID).
		Order("created_at DESC").
		Find(&databases).Error

	return databases, err
}

// FindByNameAndServer finds a database by name and server ID
func (r *Repository) FindByNameAndServer(ctx context.Context, name, serverID string) (*Database, error) {
	var database Database

	err := r.db.WithContext(ctx).
		Preload("Users").
		First(&database, "name = ? AND server_id = ?", name, serverID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDatabaseNotFound
		}

		return nil, err
	}

	return &database, nil
}

// FindByUser finds all databases for a database user
func (r *Repository) FindByUser(ctx context.Context, userID string) ([]Database, error) {
	var databases []Database

	err := r.db.WithContext(ctx).
		Joins("JOIN database_database_user ON database_database_user.database_id = databases.id").
		Where("database_database_user.database_user_id = ?", userID).
		Find(&databases).Error

	return databases, err
}

// Update updates a database
func (r *Repository) Update(ctx context.Context, database *Database) error {
	return r.db.WithContext(ctx).Save(database).Error
}

// UpdateFields updates specific fields of a database
func (r *Repository) UpdateFields(ctx context.Context, id string, fields map[string]interface{}) error {
	return r.db.WithContext(ctx).
		Model(&Database{}).
		Where("id = ?", id).
		Updates(fields).Error
}

// Delete deletes a database
func (r *Repository) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&Database{}, "id = ?", id).Error
}

// ExistsByNameAndServer checks if a database exists with the given name on the server
func (r *Repository) ExistsByNameAndServer(ctx context.Context, name, serverID string) (bool, error) {
	var count int64

	err := r.db.WithContext(ctx).
		Model(&Database{}).
		Where("name = ? AND server_id = ?", name, serverID).
		Count(&count).Error

	return count > 0, err
}

// AttachUser attaches a database user to a database
func (r *Repository) AttachUser(ctx context.Context, databaseID, userID string) error {
	return r.db.WithContext(ctx).
		Create(&DatabaseDatabaseUser{
			DatabaseID:     databaseID,
			DatabaseUserID: userID,
		}).Error
}

// DetachUser detaches a database user from a database
func (r *Repository) DetachUser(ctx context.Context, databaseID, userID string) error {
	return r.db.WithContext(ctx).
		Where("database_id = ? AND database_user_id = ?", databaseID, userID).
		Delete(&DatabaseDatabaseUser{}).Error
}

// DetachAllUsers detaches all users from a database
func (r *Repository) DetachAllUsers(ctx context.Context, databaseID string) error {
	return r.db.WithContext(ctx).
		Where("database_id = ?", databaseID).
		Delete(&DatabaseDatabaseUser{}).Error
}

// DatabaseUser operations

// CreateUser creates a new database user
func (r *Repository) CreateUser(ctx context.Context, user *DatabaseUser) error {
	return r.db.WithContext(ctx).Create(user).Error
}

// FindUserByID finds a database user by ID
func (r *Repository) FindUserByID(ctx context.Context, id string) (*DatabaseUser, error) {
	var user DatabaseUser

	err := r.db.WithContext(ctx).
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

// FindUserByIDAndServer finds a database user by ID and server ID
func (r *Repository) FindUserByIDAndServer(ctx context.Context, id, serverID string) (*DatabaseUser, error) {
	var user DatabaseUser

	err := r.db.WithContext(ctx).
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

// FindUsersByServer finds all database users for a server
func (r *Repository) FindUsersByServer(ctx context.Context, serverID string) ([]DatabaseUser, error) {
	var users []DatabaseUser

	err := r.db.WithContext(ctx).
		Preload("Databases").
		Where("server_id = ?", serverID).
		Order("created_at DESC").
		Find(&users).Error

	return users, err
}

// FindUserByNameAndServer finds a database user by name and server ID
func (r *Repository) FindUserByNameAndServer(ctx context.Context, name, serverID string) (*DatabaseUser, error) {
	var user DatabaseUser

	err := r.db.WithContext(ctx).
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

// FindUsersByDatabase finds all database users for a database
func (r *Repository) FindUsersByDatabase(ctx context.Context, databaseID string) ([]DatabaseUser, error) {
	var users []DatabaseUser

	err := r.db.WithContext(ctx).
		Joins("JOIN database_database_user ON database_database_user.database_user_id = database_users.id").
		Where("database_database_user.database_id = ?", databaseID).
		Find(&users).Error

	return users, err
}

// UpdateUser updates a database user
func (r *Repository) UpdateUser(ctx context.Context, user *DatabaseUser) error {
	return r.db.WithContext(ctx).Save(user).Error
}

// UpdateUserFields updates specific fields of a database user
func (r *Repository) UpdateUserFields(ctx context.Context, id string, fields map[string]interface{}) error {
	return r.db.WithContext(ctx).
		Model(&DatabaseUser{}).
		Where("id = ?", id).
		Updates(fields).Error
}

// DeleteUser deletes a database user
func (r *Repository) DeleteUser(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&DatabaseUser{}, "id = ?", id).Error
}

// UserExistsByNameAndServer checks if a database user exists with the given name on the server
func (r *Repository) UserExistsByNameAndServer(ctx context.Context, name, serverID string) (bool, error) {
	var count int64

	err := r.db.WithContext(ctx).
		Model(&DatabaseUser{}).
		Where("name = ? AND server_id = ?", name, serverID).
		Count(&count).Error

	return count > 0, err
}

// SyncUserDatabases syncs the databases attached to a user
func (r *Repository) SyncUserDatabases(ctx context.Context, userID string, databaseIDs []string) error {
	// Delete existing associations
	if err := r.db.WithContext(ctx).
		Where("database_user_id = ?", userID).
		Delete(&DatabaseDatabaseUser{}).Error; err != nil {
		return err
	}

	// Create new associations
	for _, dbID := range databaseIDs {
		if err := r.db.WithContext(ctx).
			Create(&DatabaseDatabaseUser{
				DatabaseID:     dbID,
				DatabaseUserID: userID,
			}).Error; err != nil {
			return err
		}
	}

	return nil
}

// FindRootUser finds the root user for a server
func (r *Repository) FindRootUser(ctx context.Context, serverID string) (*DatabaseUser, error) {
	return r.FindUserByNameAndServer(ctx, "root", serverID)
}
