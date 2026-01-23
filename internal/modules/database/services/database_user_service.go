package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/hibiken/asynq"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/database/dto"
	"github.com/kkz6/launch-go/internal/modules/database/jobs"
	"github.com/kkz6/launch-go/internal/modules/database/models"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
)

// CreateDatabaseUser creates a new database user
func (s *Service) CreateDatabaseUser(ctx context.Context, serverID, teamID string, req *dto.CreateDatabaseUserRequest, userID *string) (*models.DatabaseUser, error) {
	// Check if user name already exists
	exists, err := s.repos.User().ExistsByNameAndServer(ctx, req.Name, serverID)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing user: %w", err)
	}

	if exists {
		return nil, ErrDatabaseUserNameExists
	}

	// Validate database IDs belong to server in a single query
	if len(req.Databases) > 0 {
		validDBs, err := s.repos.Database().FindByIDsAndServer(ctx, req.Databases, serverID)
		if err != nil {
			return nil, fmt.Errorf("failed to validate databases: %w", err)
		}

		if len(validDBs) != len(req.Databases) {
			return nil, errors.New("one or more databases not found on this server")
		}
	}

	password := &dbtype.EncryptedNullableString{}
	password.Set(req.Password)

	dbUser := &models.DatabaseUser{
		Name:     req.Name,
		Password: password,
	}
	dbUser.ServerID = serverID
	dbUser.TeamID = teamID

	err = s.WithTransaction(ctx, func(tx *gorm.DB) error {
		if err := tx.Create(dbUser).Error; err != nil {
			return fmt.Errorf("failed to create database user: %w", err)
		}

		for _, dbID := range req.Databases {
			if err := tx.Create(&models.DatabaseDatabaseUser{
				DatabaseID:     dbID,
				DatabaseUserID: dbUser.ID,
			}).Error; err != nil {
				return fmt.Errorf("failed to attach database %s to user: %w", dbID, err)
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	// Dispatch job to create user on server
	s.dispatchCreateDatabaseUser(ctx, dbUser, req.Password, userID)

	return dbUser, nil
}

// GetDatabaseUser retrieves a database user by ID
func (s *Service) GetDatabaseUser(ctx context.Context, id, serverID string) (*models.DatabaseUser, error) {
	return s.repos.User().FindByIDAndServer(ctx, id, serverID)
}

// ListDatabaseUsers lists all database users for a server
func (s *Service) ListDatabaseUsers(ctx context.Context, serverID string) ([]models.DatabaseUser, error) {
	return s.repos.User().FindByServer(ctx, serverID)
}

// UpdateDatabaseUser updates a database user
func (s *Service) UpdateDatabaseUser(ctx context.Context, id, serverID string, req *dto.UpdateDatabaseUserRequest, userID *string) (*models.DatabaseUser, error) {
	dbUser, err := s.repos.User().FindByIDAndServer(ctx, id, serverID)
	if err != nil {
		return nil, err
	}

	if dbUser.IsUninstalling() {
		return nil, ErrUserBeingUninstalled
	}

	// Validate database IDs belong to server in a single query
	if len(req.Databases) > 0 {
		validDBs, err := s.repos.Database().FindByIDsAndServer(ctx, req.Databases, serverID)
		if err != nil {
			return nil, fmt.Errorf("failed to validate databases: %w", err)
		}

		if len(validDBs) != len(req.Databases) {
			return nil, errors.New("one or more databases not found on this server")
		}
	}

	// Update password
	password := &dbtype.EncryptedNullableString{}
	password.Set(req.Password)
	dbUser.Password = password

	if err := s.repos.User().Update(ctx, dbUser); err != nil {
		return nil, fmt.Errorf("failed to update database user: %w", err)
	}

	// Sync databases
	if err := s.repos.User().SyncDatabases(ctx, dbUser.ID, req.Databases); err != nil {
		return nil, fmt.Errorf("failed to sync user databases: %w", err)
	}

	// Dispatch job to update user on server
	s.dispatchUpdateDatabaseUser(ctx, dbUser, &req.Password, userID)

	// Reload user with databases
	dbUser, err = s.repos.User().FindByID(ctx, dbUser.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to reload database user: %w", err)
	}

	return dbUser, nil
}

// DeleteDatabaseUser deletes a database user from a server
func (s *Service) DeleteDatabaseUser(ctx context.Context, id, serverID string, userID *string) error {
	dbUser, err := s.repos.User().FindByIDAndServer(ctx, id, serverID)
	if err != nil {
		return err
	}

	if dbUser.IsUninstalling() {
		return ErrUserBeingUninstalled
	}

	// Mark as uninstalling
	if err := s.repos.User().MarkAsUninstalling(ctx, dbUser.ID); err != nil {
		return fmt.Errorf("failed to update user status: %w", err)
	}

	// Dispatch uninstall job
	s.dispatchDeleteDatabaseUser(ctx, dbUser, userID)

	return nil
}

// BroadcastDatabaseUserStatus broadcasts a database user status update
func (s *Service) BroadcastDatabaseUserStatus(serverID, userID, status, message string) {
	s.BroadcastToServer(serverID, "database_user.status", map[string]interface{}{
		"server_id": serverID,
		"user_id":   userID,
		"status":    status,
		"message":   message,
	})
}

// Helper methods

func (s *Service) dispatchCreateDatabaseUser(ctx context.Context, user *models.DatabaseUser, password string, userID *string) {
	s.DispatchTask("InstallDatabaseUser", func() (*asynq.Task, error) {
		return jobs.NewInstallDatabaseUserTask(user.ID, password, userID)
	}, "user_id", user.ID)
}

func (s *Service) dispatchUpdateDatabaseUser(ctx context.Context, user *models.DatabaseUser, password *string, userID *string) {
	s.DispatchTask("UpdateDatabaseUser", func() (*asynq.Task, error) {
		return jobs.NewUpdateDatabaseUserTask(user.ID, password, userID)
	}, "user_id", user.ID)
}

func (s *Service) dispatchDeleteDatabaseUser(ctx context.Context, user *models.DatabaseUser, userID *string) {
	s.DispatchTask("UninstallDatabaseUser", func() (*asynq.Task, error) {
		return jobs.NewUninstallDatabaseUserTask(user.ID, userID)
	}, "user_id", user.ID)
}
