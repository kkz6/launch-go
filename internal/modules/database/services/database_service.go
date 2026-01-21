package services

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/database/dto"
	"github.com/kkz6/launch-go/internal/modules/database/jobs"
	"github.com/kkz6/launch-go/internal/modules/database/models"
	"github.com/kkz6/launch-go/internal/pkg/activity"
)

// CreateDatabase creates a new database on a server
func (s *Service) CreateDatabase(ctx context.Context, serverID, teamID string, req *dto.CreateDatabaseRequest, userID *string) (*models.Database, error) {
	// Check if database name already exists
	exists, err := s.repos.Database().ExistsByNameAndServer(ctx, req.Name, serverID)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing database: %w", err)
	}

	if exists {
		return nil, ErrDatabaseNameExists
	}

	// Create database record
	database := &models.Database{
		Name: req.Name,
	}
	database.ServerID = serverID
	database.TeamID = teamID

	if err := s.repos.Database().Create(ctx, database); err != nil {
		return nil, fmt.Errorf("failed to create database: %w", err)
	}

	uid := ""
	if userID != nil {
		uid = *userID
	}
	activity.LogEvent(ctx, s.repos.DB(), "created", uid, database, "Database was created")

	// Attach root user if exists
	s.attachRootUser(ctx, serverID, database.ID)

	// Handle existing user attachment
	if !req.CreateUser && req.ExistingUserID != nil && *req.ExistingUserID != "" {
		existingUser, err := s.repos.User().FindByIDAndServer(ctx, *req.ExistingUserID, serverID)
		if err != nil {
			s.LogWarn("Failed to find existing user", "user_id", *req.ExistingUserID, "error", err)
		}
		if err == nil {
			if err := s.repos.Database().AttachUser(ctx, database.ID, existingUser.ID); err != nil {
				s.LogError(err, "Failed to attach existing user to database")
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
	userExists, err := s.repos.User().ExistsByNameAndServer(ctx, req.UserName, serverID)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing user: %w", err)
	}

	if userExists {
		return nil, ErrDatabaseUserNameExists
	}

	// Create database user
	dbUser := &models.DatabaseUser{
		Name:     req.UserName,
		Password: &req.UserPassword,
	}
	dbUser.ServerID = serverID
	dbUser.TeamID = teamID

	if err := s.repos.User().Create(ctx, dbUser); err != nil {
		return nil, fmt.Errorf("failed to create database user: %w", err)
	}

	// Attach user to database
	if err := s.repos.Database().AttachUser(ctx, database.ID, dbUser.ID); err != nil {
		return nil, fmt.Errorf("failed to attach user to database: %w", err)
	}

	// Dispatch jobs to create database and user
	s.dispatchCreateDatabaseWithNewUser(ctx, database, dbUser, req.UserPassword, userID)

	return database, nil
}

// GetDatabase retrieves a database by ID
func (s *Service) GetDatabase(ctx context.Context, id, serverID, teamID string) (*models.Database, error) {
	return s.repos.Database().FindByIDAndServerAndTeam(ctx, id, serverID, teamID)
}

// ListDatabases lists all databases for a server
func (s *Service) ListDatabases(ctx context.Context, serverID, teamID string) ([]models.Database, error) {
	return s.repos.Database().FindByServerAndTeam(ctx, serverID, teamID)
}

// DeleteDatabase deletes a database from a server
func (s *Service) DeleteDatabase(ctx context.Context, id, serverID, teamID string, userID *string) error {
	database, err := s.repos.Database().FindByIDAndServerAndTeam(ctx, id, serverID, teamID)
	if err != nil {
		return err
	}

	if database.IsUninstalling() {
		return ErrDatabaseBeingUninstalled
	}

	uid := ""
	if userID != nil {
		uid = *userID
	}
	activity.LogEvent(ctx, s.repos.DB(), "deleted", uid, database, "Database deletion requested")

	// Mark as uninstalling
	database.MarkAsUninstalling()
	if err := s.repos.Database().Update(ctx, database); err != nil {
		return fmt.Errorf("failed to update database status: %w", err)
	}

	// Dispatch uninstall job
	s.dispatchDeleteDatabase(ctx, database, userID)

	return nil
}

// SyncDatabases syncs databases from the server
func (s *Service) SyncDatabases(ctx context.Context, serverID string, userID *string) error {
	if !s.HasQueue() {
		// In test mode without queue, just return success
		return nil
	}

	task, err := jobs.NewSyncDatabasesTask(serverID, userID)
	if err != nil {
		return fmt.Errorf("failed to create sync task: %w", err)
	}

	if err := s.EnqueueTask(task); err != nil {
		s.LogError(err, "Failed to enqueue sync databases job", "server_id", serverID)
		return fmt.Errorf("failed to enqueue sync job: %w", err)
	}

	return nil
}

// BroadcastDatabaseStatus broadcasts a database status update
func (s *Service) BroadcastDatabaseStatus(serverID, databaseID, status, message string) {
	s.BroadcastToServer(serverID, "database.status", map[string]interface{}{
		"server_id":   serverID,
		"database_id": databaseID,
		"status":      status,
		"message":     message,
	})
}

// Helper methods

func (s *Service) attachRootUser(ctx context.Context, serverID, databaseID string) {
	rootUser, err := s.repos.User().FindRootUser(ctx, serverID)
	if err != nil {
		return
	}

	if err := s.repos.Database().AttachUser(ctx, databaseID, rootUser.ID); err != nil {
		s.LogError(err, "Failed to attach root user to database", "database_id", databaseID)
	}
}

func (s *Service) dispatchCreateDatabase(ctx context.Context, database *models.Database, userID *string) {
	s.DispatchTask("InstallDatabase", func() (*asynq.Task, error) {
		return jobs.NewInstallDatabaseTask(database.ID, userID)
	}, "database_id", database.ID)
}

func (s *Service) dispatchCreateDatabaseWithExistingUser(ctx context.Context, database *models.Database, user *models.DatabaseUser, userID *string) {
	if !s.HasQueue() {
		return
	}

	// Create database first, then update user permissions
	installTask, err := jobs.NewInstallDatabaseTask(database.ID, userID)
	if err != nil {
		s.LogError(err, "Failed to create install database task", "database_id", database.ID)
		return
	}

	if err := s.EnqueueTask(installTask); err != nil {
		s.LogError(err, "Failed to enqueue install database job", "database_id", database.ID)
	}

	// Update user permissions
	updateTask, err := jobs.NewUpdateDatabaseUserTask(user.ID, nil, userID)
	if err != nil {
		s.LogError(err, "Failed to create update user task", "user_id", user.ID)
		return
	}

	if err := s.EnqueueTask(updateTask); err != nil {
		s.LogError(err, "Failed to enqueue update user job", "user_id", user.ID)
	}
}

func (s *Service) dispatchCreateDatabaseWithNewUser(ctx context.Context, database *models.Database, user *models.DatabaseUser, password string, userID *string) {
	if !s.HasQueue() {
		return
	}

	// Create database first
	installDbTask, err := jobs.NewInstallDatabaseTask(database.ID, userID)
	if err != nil {
		s.LogError(err, "Failed to create install database task", "database_id", database.ID)
		return
	}

	if err := s.EnqueueTask(installDbTask); err != nil {
		s.LogError(err, "Failed to enqueue install database job", "database_id", database.ID)
	}

	// Then create user
	installUserTask, err := jobs.NewInstallDatabaseUserTask(user.ID, password, userID)
	if err != nil {
		s.LogError(err, "Failed to create install user task", "user_id", user.ID)
		return
	}

	if err := s.EnqueueTask(installUserTask); err != nil {
		s.LogError(err, "Failed to enqueue install user job", "user_id", user.ID)
	}
}

func (s *Service) dispatchDeleteDatabase(ctx context.Context, database *models.Database, userID *string) {
	s.DispatchTask("UninstallDatabase", func() (*asynq.Task, error) {
		return jobs.NewUninstallDatabaseTask(database.ID, userID)
	}, "database_id", database.ID)
}
