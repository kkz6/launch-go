package services

import (
	"context"
	"fmt"

	"github.com/kkz6/launch-go/internal/modules/database/dto"
	"github.com/kkz6/launch-go/internal/modules/database/jobs"
	"github.com/kkz6/launch-go/internal/modules/database/models"
)

// CreateDatabaseUser creates a new database user
func (s *Service) CreateDatabaseUser(ctx context.Context, serverID string, req *dto.CreateDatabaseUserRequest, userID *string) (*models.DatabaseUser, error) {
	// Check if user name already exists
	exists, err := s.repo.UserExistsByNameAndServer(ctx, req.Name, serverID)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing user: %w", err)
	}

	if exists {
		return nil, ErrDatabaseUserNameExists
	}

	// Validate database IDs belong to server
	if len(req.Databases) > 0 {
		for _, dbID := range req.Databases {
			_, err := s.repo.FindByIDAndServer(ctx, dbID, serverID)
			if err != nil {
				return nil, fmt.Errorf("database %s not found on server: %w", dbID, err)
			}
		}
	}

	// Create database user
	dbUser := &models.DatabaseUser{
		ServerID: serverID,
		Name:     req.Name,
		Password: req.Password,
	}

	if err := s.repo.CreateUser(ctx, dbUser); err != nil {
		return nil, fmt.Errorf("failed to create database user: %w", err)
	}

	// Attach databases
	for _, dbID := range req.Databases {
		if err := s.repo.AttachUser(ctx, dbID, dbUser.ID); err != nil {
			s.logger.Error().Err(err).Str("database_id", dbID).Str("user_id", dbUser.ID).Msg("Failed to attach database to user")
		}
	}

	// Dispatch job to create user on server
	s.dispatchCreateDatabaseUser(ctx, dbUser, req.Password, userID)

	return dbUser, nil
}

// GetDatabaseUser retrieves a database user by ID
func (s *Service) GetDatabaseUser(ctx context.Context, id, serverID string) (*models.DatabaseUser, error) {
	return s.repo.FindUserByIDAndServer(ctx, id, serverID)
}

// ListDatabaseUsers lists all database users for a server
func (s *Service) ListDatabaseUsers(ctx context.Context, serverID string) ([]models.DatabaseUser, error) {
	return s.repo.FindUsersByServer(ctx, serverID)
}

// UpdateDatabaseUser updates a database user
func (s *Service) UpdateDatabaseUser(ctx context.Context, id, serverID string, req *dto.UpdateDatabaseUserRequest, userID *string) (*models.DatabaseUser, error) {
	dbUser, err := s.repo.FindUserByIDAndServer(ctx, id, serverID)
	if err != nil {
		return nil, err
	}

	if dbUser.IsUninstalling() {
		return nil, ErrUserBeingUninstalled
	}

	// Validate database IDs belong to server
	if len(req.Databases) > 0 {
		for _, dbID := range req.Databases {
			_, err := s.repo.FindByIDAndServer(ctx, dbID, serverID)
			if err != nil {
				return nil, fmt.Errorf("database %s not found on server: %w", dbID, err)
			}
		}
	}

	// Update password
	dbUser.Password = req.Password
	if err := s.repo.UpdateUser(ctx, dbUser); err != nil {
		return nil, fmt.Errorf("failed to update database user: %w", err)
	}

	// Sync databases
	if err := s.repo.SyncUserDatabases(ctx, dbUser.ID, req.Databases); err != nil {
		return nil, fmt.Errorf("failed to sync user databases: %w", err)
	}

	// Dispatch job to update user on server
	s.dispatchUpdateDatabaseUser(ctx, dbUser, &req.Password, userID)

	// Reload user with databases
	dbUser, _ = s.repo.FindUserByID(ctx, dbUser.ID)

	return dbUser, nil
}

// DeleteDatabaseUser deletes a database user from a server
func (s *Service) DeleteDatabaseUser(ctx context.Context, id, serverID string, userID *string) error {
	dbUser, err := s.repo.FindUserByIDAndServer(ctx, id, serverID)
	if err != nil {
		return err
	}

	if dbUser.IsUninstalling() {
		return ErrUserBeingUninstalled
	}

	// Mark as uninstalling
	dbUser.MarkAsUninstalling()
	if err := s.repo.UpdateUser(ctx, dbUser); err != nil {
		return fmt.Errorf("failed to update user status: %w", err)
	}

	// Dispatch uninstall job
	s.dispatchDeleteDatabaseUser(ctx, dbUser, userID)

	return nil
}

// BroadcastDatabaseUserStatus broadcasts a database user status update
func (s *Service) BroadcastDatabaseUserStatus(serverID, userID, status, message string) {
	s.ws.BroadcastToServer(serverID, "database_user.status", map[string]interface{}{
		"server_id": serverID,
		"user_id":   userID,
		"status":    status,
		"message":   message,
	})
}

// Helper methods

func (s *Service) dispatchCreateDatabaseUser(ctx context.Context, user *models.DatabaseUser, password string, userID *string) {
	if s.queue == nil {
		return
	}

	task, err := jobs.NewInstallDatabaseUserTask(user.ID, password, userID)
	if err != nil {
		s.logger.Error().Err(err).Str("user_id", user.ID).Msg("Failed to create install user task")

		return
	}

	if _, err := s.queue.EnqueueDefault(task); err != nil {
		s.logger.Error().Err(err).Str("user_id", user.ID).Msg("Failed to enqueue install user job")
	}
}

func (s *Service) dispatchUpdateDatabaseUser(ctx context.Context, user *models.DatabaseUser, password *string, userID *string) {
	if s.queue == nil {
		return
	}

	task, err := jobs.NewUpdateDatabaseUserTask(user.ID, password, userID)
	if err != nil {
		s.logger.Error().Err(err).Str("user_id", user.ID).Msg("Failed to create update user task")

		return
	}

	if _, err := s.queue.EnqueueDefault(task); err != nil {
		s.logger.Error().Err(err).Str("user_id", user.ID).Msg("Failed to enqueue update user job")
	}
}

func (s *Service) dispatchDeleteDatabaseUser(ctx context.Context, user *models.DatabaseUser, userID *string) {
	if s.queue == nil {
		return
	}

	task, err := jobs.NewUninstallDatabaseUserTask(user.ID, userID)
	if err != nil {
		s.logger.Error().Err(err).Str("user_id", user.ID).Msg("Failed to create uninstall user task")

		return
	}

	if _, err := s.queue.EnqueueDefault(task); err != nil {
		s.logger.Error().Err(err).Str("user_id", user.ID).Msg("Failed to enqueue uninstall user job")
	}
}
