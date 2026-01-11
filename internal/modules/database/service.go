package database

import (
	"context"
	"errors"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/kkz6/launch-go/internal/modules/database/jobs"
	"github.com/kkz6/launch-go/internal/queue"
	"github.com/kkz6/launch-go/internal/websocket"
)

var (
	ErrServerNotFound          = errors.New("server not found")
	ErrDatabaseNameExists      = errors.New("a database with this name already exists on this server")
	ErrDatabaseUserNameExists  = errors.New("a database user with this name already exists on this server")
	ErrInvalidExistingUser     = errors.New("the specified existing user was not found")
	ErrDatabaseBeingUninstalled = errors.New("database is being uninstalled")
	ErrUserBeingUninstalled    = errors.New("user is being uninstalled")
)

// ServerRepository defines the interface for server operations needed by the database service
type ServerRepository interface {
	FindByID(ctx context.Context, id string) (interface{}, error)
	FindByIDAndTeam(ctx context.Context, id, teamID string) (interface{}, error)
}

// Service handles business logic for database operations
type Service struct {
	repo         *Repository
	serverRepo   ServerRepository
	queue        *queue.Client
	ws           *websocket.Hub
	logger       *zerolog.Logger
}

// NewService creates a new database service
func NewService(repo *Repository, serverRepo ServerRepository, queue *queue.Client, ws *websocket.Hub, logger *zerolog.Logger) *Service {
	return &Service{
		repo:       repo,
		serverRepo: serverRepo,
		queue:      queue,
		ws:         ws,
		logger:     logger,
	}
}

// CreateDatabase creates a new database on a server
func (s *Service) CreateDatabase(ctx context.Context, serverID string, req *CreateDatabaseRequest, userID *string) (*Database, error) {
	// Check if database name already exists
	exists, err := s.repo.ExistsByNameAndServer(ctx, req.Name, serverID)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing database: %w", err)
	}

	if exists {
		return nil, ErrDatabaseNameExists
	}

	// Create database record
	database := &Database{
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
	dbUser := &DatabaseUser{
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
func (s *Service) GetDatabase(ctx context.Context, id, serverID string) (*Database, error) {
	return s.repo.FindByIDAndServer(ctx, id, serverID)
}

// ListDatabases lists all databases for a server
func (s *Service) ListDatabases(ctx context.Context, serverID string) ([]Database, error) {
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

// Database User operations

// CreateDatabaseUser creates a new database user
func (s *Service) CreateDatabaseUser(ctx context.Context, serverID string, req *CreateDatabaseUserRequest, userID *string) (*DatabaseUser, error) {
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
	dbUser := &DatabaseUser{
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
func (s *Service) GetDatabaseUser(ctx context.Context, id, serverID string) (*DatabaseUser, error) {
	return s.repo.FindUserByIDAndServer(ctx, id, serverID)
}

// ListDatabaseUsers lists all database users for a server
func (s *Service) ListDatabaseUsers(ctx context.Context, serverID string) ([]DatabaseUser, error) {
	return s.repo.FindUsersByServer(ctx, serverID)
}

// UpdateDatabaseUser updates a database user
func (s *Service) UpdateDatabaseUser(ctx context.Context, id, serverID string, req *UpdateDatabaseUserRequest, userID *string) (*DatabaseUser, error) {
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

func (s *Service) dispatchCreateDatabase(ctx context.Context, database *Database, userID *string) {
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

func (s *Service) dispatchCreateDatabaseWithExistingUser(ctx context.Context, database *Database, user *DatabaseUser, userID *string) {
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

func (s *Service) dispatchCreateDatabaseWithNewUser(ctx context.Context, database *Database, user *DatabaseUser, password string, userID *string) {
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

func (s *Service) dispatchDeleteDatabase(ctx context.Context, database *Database, userID *string) {
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

func (s *Service) dispatchCreateDatabaseUser(ctx context.Context, user *DatabaseUser, password string, userID *string) {
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

func (s *Service) dispatchUpdateDatabaseUser(ctx context.Context, user *DatabaseUser, password *string, userID *string) {
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

func (s *Service) dispatchDeleteDatabaseUser(ctx context.Context, user *DatabaseUser, userID *string) {
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

// BroadcastDatabaseStatus broadcasts a database status update
func (s *Service) BroadcastDatabaseStatus(serverID, databaseID, status, message string) {
	s.ws.BroadcastToServer(serverID, "database.status", map[string]interface{}{
		"server_id":   serverID,
		"database_id": databaseID,
		"status":      status,
		"message":     message,
	})
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
