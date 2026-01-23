package services

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/database/dto"
	"github.com/kkz6/launch-go/internal/modules/database/jobs"
	"github.com/kkz6/launch-go/internal/modules/database/models"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	"github.com/kkz6/launch-go/internal/pkg/launch/activity"
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

	// If creating a user, validate username doesn't exist before starting the transaction
	if req.CreateUser {
		userExists, err := s.repos.User().ExistsByNameAndServer(ctx, req.UserName, serverID)
		if err != nil {
			return nil, fmt.Errorf("failed to check existing user: %w", err)
		}

		if userExists {
			return nil, ErrDatabaseUserNameExists
		}
	}

	database := &models.Database{
		Name: req.Name,
	}
	database.ServerID = serverID
	database.TeamID = teamID

	var dbUser *models.DatabaseUser
	var existingUser *models.DatabaseUser

	err = s.WithTransaction(ctx, func(tx *gorm.DB) error {
		if err := tx.Create(database).Error; err != nil {
			return fmt.Errorf("failed to create database: %w", err)
		}

		// Attach root user if exists
		rootUser, rootErr := s.repos.User().FindRootUser(ctx, serverID)
		if rootErr == nil {
			if err := tx.Create(&models.DatabaseDatabaseUser{
				DatabaseID:     database.ID,
				DatabaseUserID: rootUser.ID,
			}).Error; err != nil {
				return fmt.Errorf("failed to attach root user to database: %w", err)
			}
		}

		// Handle existing user attachment
		if !req.CreateUser && req.ExistingUserID != nil && *req.ExistingUserID != "" {
			user, err := s.repos.User().FindByIDAndServer(ctx, *req.ExistingUserID, serverID)
			if err != nil {
				return fmt.Errorf("existing user not found: %w", err)
			}

			existingUser = user

			if err := tx.Create(&models.DatabaseDatabaseUser{
				DatabaseID:     database.ID,
				DatabaseUserID: existingUser.ID,
			}).Error; err != nil {
				return fmt.Errorf("failed to attach existing user to database: %w", err)
			}

			return nil
		}

		// If creating a new user
		if req.CreateUser {
			password := &dbtype.EncryptedNullableString{}
			password.Set(req.UserPassword)

			dbUser = &models.DatabaseUser{
				Name:     req.UserName,
				Password: password,
			}
			dbUser.ServerID = serverID
			dbUser.TeamID = teamID

			if err := tx.Create(dbUser).Error; err != nil {
				return fmt.Errorf("failed to create database user: %w", err)
			}

			if err := tx.Create(&models.DatabaseDatabaseUser{
				DatabaseID:     database.ID,
				DatabaseUserID: dbUser.ID,
			}).Error; err != nil {
				return fmt.Errorf("failed to attach user to database: %w", err)
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	activity.RecordEventPtr(ctx, "created", userID, database, "Database was created")

	// Dispatch appropriate jobs after successful transaction
	if existingUser != nil {
		s.dispatchCreateDatabaseWithExistingUser(ctx, database, existingUser, userID)
	} else if dbUser != nil {
		s.dispatchCreateDatabaseWithNewUser(ctx, database, dbUser, req.UserPassword, userID)
	} else {
		s.dispatchCreateDatabase(ctx, database, userID)
	}

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

	activity.RecordEventPtr(ctx, "deleted", userID, database, "Database deletion requested")

	// Mark as uninstalling
	if err := s.repos.Database().MarkAsUninstalling(ctx, database.ID); err != nil {
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

func (s *Service) dispatchCreateDatabase(ctx context.Context, database *models.Database, userID *string) {
	s.DispatchTask("InstallDatabase", func() (*asynq.Task, error) {
		return jobs.NewInstallDatabaseTask(database.ID, userID)
	}, "database_id", database.ID)
}

func (s *Service) dispatchCreateDatabaseWithExistingUser(ctx context.Context, database *models.Database, user *models.DatabaseUser, userID *string) {
	// NOTE: These two tasks have an ordering dependency (database must exist before granting
	// user permissions). The task implementations are idempotent -- InstallDatabaseUser and
	// UpdateDatabaseUser will re-check state before executing. The asynq TaskID deduplication
	// also prevents duplicate concurrent execution. If stronger ordering is needed in the
	// future, these should be combined into a single composite task.
	s.DispatchTask("InstallDatabase", func() (*asynq.Task, error) {
		return jobs.NewInstallDatabaseTask(database.ID, userID)
	}, "database_id", database.ID)

	s.DispatchTask("UpdateDatabaseUser", func() (*asynq.Task, error) {
		return jobs.NewUpdateDatabaseUserTask(user.ID, nil, userID)
	}, "user_id", user.ID)
}

func (s *Service) dispatchCreateDatabaseWithNewUser(ctx context.Context, database *models.Database, user *models.DatabaseUser, password string, userID *string) {
	// NOTE: These two tasks have an ordering dependency (database must exist before creating
	// the user with grants on it). The task implementations are idempotent -- the install
	// database user job will re-check state and grant privileges only on databases that exist.
	// The asynq TaskID deduplication also prevents duplicate concurrent execution. If stronger
	// ordering is needed in the future, these should be combined into a single composite task.
	s.DispatchTask("InstallDatabase", func() (*asynq.Task, error) {
		return jobs.NewInstallDatabaseTask(database.ID, userID)
	}, "database_id", database.ID)

	s.DispatchTask("InstallDatabaseUser", func() (*asynq.Task, error) {
		return jobs.NewInstallDatabaseUserTask(user.ID, password, userID)
	}, "user_id", user.ID)
}

func (s *Service) dispatchDeleteDatabase(ctx context.Context, database *models.Database, userID *string) {
	s.DispatchTask("UninstallDatabase", func() (*asynq.Task, error) {
		return jobs.NewUninstallDatabaseTask(database.ID, userID)
	}, "database_id", database.ID)
}
