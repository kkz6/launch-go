package services

import (
	"context"
	"fmt"

	"github.com/kkz6/launch-go/internal/modules/database/dto"
	"github.com/kkz6/launch-go/internal/modules/database/jobs"
	"github.com/kkz6/launch-go/internal/modules/database/models"
)

// CreateDatabase creates a new database on a server
func (s *Service) CreateDatabase(ctx context.Context, serverID string, req *dto.CreateDatabaseRequest, userID *string) (*models.Database, error) {
	// Check if database name already exists
	exists, err := s.repo.ExistsByNameAndServer(ctx, req.Name, serverID)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing database: %w", err)
	}

	if exists {
		return nil, ErrDatabaseNameExists
	}

	// Create database record
	database := &models.Database{
		ServerID: serverID,
		Name:     req.Name,
	}

	if err := s.repo.Create(ctx, database); err != nil {
		return nil, fmt.Errorf("failed to create database: %w", err)
	}

	// Attach root user if exists
	s.attachRootUser(ctx, serverID, database.ID)

	// Handle existing user attachment
	if !req.CreateUser && req.ExistingUserID != nil && *req.ExistingUserID != "" {
		existingUser, err := s.repo.FindUserByIDAndServer(ctx, *req.ExistingUserID, serverID)
		if err != nil {
			s.logger.Warn().Err(err).Str("user_id", *req.ExistingUserID).Msg("Failed to find existing user")
		} else {
			if err := s.repo.AttachUser(ctx, database.ID, existingUser.ID); err != nil {
				s.logger.Error().Err(err).Msg("Failed to attach existing user to database")
			}

			// Dispatch job to create database and update user permissions
			s.dispatchCreateDatabaseWithExistingUser(ctx, database, existingUser, userID)

			return database, nil
		}
	}

	// If not creating a user, just dispatch database creation
	if !req.CreateUser {
		s.dispatchCreateDatabase(ctx, database, userID)

		return database, nil
	}

	// Check if user name already exists
	userExists, err := s.repo.UserExistsByNameAndServer(ctx, req.UserName, serverID)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing user: %w", err)
	}

	if userExists {
		return nil, ErrDatabaseUserNameExists
	}

	// Create database user
	dbUser := &models.DatabaseUser{
		ServerID: serverID,
		Name:     req.UserName,
		Password: req.UserPassword,
	}

	if err := s.repo.CreateUser(ctx, dbUser); err != nil {
		return nil, fmt.Errorf("failed to create database user: %w", err)
	}

	// Attach user to database
	if err := s.repo.AttachUser(ctx, database.ID, dbUser.ID); err != nil {
		return nil, fmt.Errorf("failed to attach user to database: %w", err)
	}

	// Dispatch jobs to create database and user
	s.dispatchCreateDatabaseWithNewUser(ctx, database, dbUser, req.UserPassword, userID)

	return database, nil
}

// GetDatabase retrieves a database by ID
func (s *Service) GetDatabase(ctx context.Context, id, serverID string) (*models.Database, error) {
	return s.repo.FindByIDAndServer(ctx, id, serverID)
}

// ListDatabases lists all databases for a server
func (s *Service) ListDatabases(ctx context.Context, serverID string) ([]models.Database, error) {
	return s.repo.FindByServer(ctx, serverID)
}

// DeleteDatabase deletes a database from a server
func (s *Service) DeleteDatabase(ctx context.Context, id, serverID string, userID *string) error {
	database, err := s.repo.FindByIDAndServer(ctx, id, serverID)
	if err != nil {
		return err
	}

	if database.IsUninstalling() {
		return ErrDatabaseBeingUninstalled
	}

	// Mark as uninstalling
	database.MarkAsUninstalling()
	if err := s.repo.Update(ctx, database); err != nil {
		return fmt.Errorf("failed to update database status: %w", err)
	}

	// Dispatch uninstall job
	s.dispatchDeleteDatabase(ctx, database, userID)

	return nil
}

// SyncDatabases syncs databases from the server
func (s *Service) SyncDatabases(ctx context.Context, serverID string, userID *string) error {
	if s.queue == nil {
		// In test mode without queue, just return success
		return nil
	}

	task, err := jobs.NewSyncDatabasesTask(serverID, userID)
	if err != nil {
		return fmt.Errorf("failed to create sync task: %w", err)
	}

	if _, err := s.queue.EnqueueDefault(task); err != nil {
		s.logger.Error().Err(err).Str("server_id", serverID).Msg("Failed to enqueue sync databases job")

		return fmt.Errorf("failed to enqueue sync job: %w", err)
	}

	return nil
}

// BroadcastDatabaseStatus broadcasts a database status update
func (s *Service) BroadcastDatabaseStatus(serverID, databaseID, status, message string) {
	s.ws.BroadcastToServer(serverID, "database.status", map[string]interface{}{
		"server_id":   serverID,
		"database_id": databaseID,
		"status":      status,
		"message":     message,
	})
}

// Helper methods

func (s *Service) attachRootUser(ctx context.Context, serverID, databaseID string) {
	rootUser, err := s.repo.FindRootUser(ctx, serverID)
	if err != nil {
		return
	}

	if err := s.repo.AttachUser(ctx, databaseID, rootUser.ID); err != nil {
		s.logger.Error().Err(err).Str("database_id", databaseID).Msg("Failed to attach root user to database")
	}
}

func (s *Service) dispatchCreateDatabase(ctx context.Context, database *models.Database, userID *string) {
	if s.queue == nil {
		return
	}

	task, err := jobs.NewInstallDatabaseTask(database.ID, userID)
	if err != nil {
		s.logger.Error().Err(err).Str("database_id", database.ID).Msg("Failed to create install database task")

		return
	}

	if _, err := s.queue.EnqueueDefault(task); err != nil {
		s.logger.Error().Err(err).Str("database_id", database.ID).Msg("Failed to enqueue install database job")
	}
}

func (s *Service) dispatchCreateDatabaseWithExistingUser(ctx context.Context, database *models.Database, user *models.DatabaseUser, userID *string) {
	if s.queue == nil {
		return
	}

	// Create database first, then update user permissions
	installTask, err := jobs.NewInstallDatabaseTask(database.ID, userID)
	if err != nil {
		s.logger.Error().Err(err).Str("database_id", database.ID).Msg("Failed to create install database task")

		return
	}

	if _, err := s.queue.EnqueueDefault(installTask); err != nil {
		s.logger.Error().Err(err).Str("database_id", database.ID).Msg("Failed to enqueue install database job")
	}

	// Update user permissions
	updateTask, err := jobs.NewUpdateDatabaseUserTask(user.ID, nil, userID)
	if err != nil {
		s.logger.Error().Err(err).Str("user_id", user.ID).Msg("Failed to create update user task")

		return
	}

	if _, err := s.queue.EnqueueDefault(updateTask); err != nil {
		s.logger.Error().Err(err).Str("user_id", user.ID).Msg("Failed to enqueue update user job")
	}
}

func (s *Service) dispatchCreateDatabaseWithNewUser(ctx context.Context, database *models.Database, user *models.DatabaseUser, password string, userID *string) {
	if s.queue == nil {
		return
	}

	// Create database first
	installDbTask, err := jobs.NewInstallDatabaseTask(database.ID, userID)
	if err != nil {
		s.logger.Error().Err(err).Str("database_id", database.ID).Msg("Failed to create install database task")

		return
	}

	if _, err := s.queue.EnqueueDefault(installDbTask); err != nil {
		s.logger.Error().Err(err).Str("database_id", database.ID).Msg("Failed to enqueue install database job")
	}

	// Then create user
	installUserTask, err := jobs.NewInstallDatabaseUserTask(user.ID, password, userID)
	if err != nil {
		s.logger.Error().Err(err).Str("user_id", user.ID).Msg("Failed to create install user task")

		return
	}

	if _, err := s.queue.EnqueueDefault(installUserTask); err != nil {
		s.logger.Error().Err(err).Str("user_id", user.ID).Msg("Failed to enqueue install user job")
	}
}

func (s *Service) dispatchDeleteDatabase(ctx context.Context, database *models.Database, userID *string) {
	if s.queue == nil {
		return
	}

	task, err := jobs.NewUninstallDatabaseTask(database.ID, userID)
	if err != nil {
		s.logger.Error().Err(err).Str("database_id", database.ID).Msg("Failed to create uninstall database task")

		return
	}

	if _, err := s.queue.EnqueueDefault(task); err != nil {
		s.logger.Error().Err(err).Str("database_id", database.ID).Msg("Failed to enqueue uninstall database job")
	}
}
