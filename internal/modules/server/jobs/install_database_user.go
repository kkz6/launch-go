package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/enums"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeInstallDatabaseUser = "server:install_database_user"

// InstallDatabaseUserPayload contains data for creating a database user
type InstallDatabaseUserPayload struct {
	DatabaseUserID string  `json:"database_user_id"`
	Password       string  `json:"password"`
	UserID         *string `json:"user_id,omitempty"`
}

// InstallDatabaseUserJob handles creating a database user on a server
type InstallDatabaseUserJob struct {
	*JobContext
	jobs.InstallationTracker
}

// NewInstallDatabaseUserTask creates a new asynq task for creating a database user
func NewInstallDatabaseUserTask(databaseUserID, password string, userID *string) (*asynq.Task, error) {
	return jobs.NewTask(TypeInstallDatabaseUser, InstallDatabaseUserPayload{
		DatabaseUserID: databaseUserID,
		Password:       password,
		UserID:         userID,
	})
}

// Handle processes the install database user job
func (j *InstallDatabaseUserJob) Handle(ctx context.Context, t *asynq.Task) error {
	payload, err := jobs.UnmarshalPayload[InstallDatabaseUserPayload](t)
	if err != nil {
		return err
	}

	j.Logger.Info().
		Str("database_user_id", payload.DatabaseUserID).
		Msg("Creating database user")

	// Fetch the database user with server and databases
	dbUser, err := j.Repo.FindDatabaseUserByIDWithServer(ctx, payload.DatabaseUserID)
	if err != nil {
		return fmt.Errorf("failed to find database user: %w", err)
	}

	j.broadcastProgress(dbUser.ServerID, "creating", fmt.Sprintf("Creating database user: %s", dbUser.Name))

	// Determine the database type from installed services
	var dbService models.InstalledService
	if err := j.DB.Where("server_id = ? AND type IN (?, ?)",
		dbUser.ServerID, enums.ServiceTypeMySql, enums.ServiceTypePostgreSql).
		First(&dbService).Error; err != nil {
		return fmt.Errorf("no database service found on server: %w", err)
	}

	// Create the database user using the factory
	factory := NewDatabaseTaskFactory(enums.ServiceType(dbService.Type), dbUser.Server)
	task, err := factory.CreateUser(dbUser.Name, payload.Password)
	if err != nil {
		return err
	}

	_, err = j.RunTask(dbUser.Server, task).
		AsRoot().
		Throw().
		Dispatch(ctx)
	if err != nil {
		return fmt.Errorf("failed to create database user: %w", err)
	}

	// Mark the database user as installed
	if err := j.MarkAsInstalled(j.DB, dbUser); err != nil {
		return fmt.Errorf("failed to update database user status: %w", err)
	}

	j.broadcastProgress(dbUser.ServerID, "created", fmt.Sprintf("Database user %s created successfully", dbUser.Name))

	return nil
}

// Failed handles job failure
func (j *InstallDatabaseUserJob) Failed(ctx context.Context, t *asynq.Task, err error) {
	payload, unmarshalErr := jobs.UnmarshalPayload[InstallDatabaseUserPayload](t)
	if unmarshalErr != nil {
		j.Logger.Error().Err(unmarshalErr).Msg("Failed to unmarshal payload in failure handler")
		return
	}

	j.Logger.Error().
		Err(err).
		Str("database_user_id", payload.DatabaseUserID).
		Msg("Failed to create database user")

	// Fetch the database user to get server ID for broadcasting
	dbUser, findErr := j.Repo.FindDatabaseUserByID(ctx, payload.DatabaseUserID)
	if findErr != nil {
		return
	}

	// Mark installation as failed
	j.MarkInstallationFailed(j.DB, dbUser)

	j.broadcastProgress(dbUser.ServerID, "failed", fmt.Sprintf("Failed to create database user: %s", dbUser.Name))
}

func (j *InstallDatabaseUserJob) broadcastProgress(serverID, status, message string) {
	j.BroadcastToServer(serverID, "server.database_user.progress", map[string]interface{}{
		"server_id": serverID,
		"status":    status,
		"message":   message,
	})
}
