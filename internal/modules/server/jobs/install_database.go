package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeInstallDatabase = "server:install_database"

// InstallDatabasePayload contains data for creating a database
type InstallDatabasePayload struct {
	DatabaseID string  `json:"database_id"`
	UserID     *string `json:"user_id,omitempty"`
}

// InstallDatabaseJob handles creating a database on a server
type InstallDatabaseJob struct {
	*JobContext
	jobs.InstallationTracker
}

// NewInstallDatabaseTask creates a new asynq task for creating a database
func NewInstallDatabaseTask(databaseID string, userID *string) (*asynq.Task, error) {
	return jobs.NewTask(TypeInstallDatabase, InstallDatabasePayload{
		DatabaseID: databaseID,
		UserID:     userID,
	})
}

// Handle processes the install database job
func (j *InstallDatabaseJob) Handle(ctx context.Context, t *asynq.Task) error {
	payload, err := jobs.UnmarshalPayload[InstallDatabasePayload](t)
	if err != nil {
		return err
	}

	j.Logger.Info().
		Str("database_id", payload.DatabaseID).
		Msg("Creating database")

	// Fetch the database with server
	database, err := j.Repo.FindDatabaseByIDWithServer(ctx, payload.DatabaseID)
	if err != nil {
		return fmt.Errorf("failed to find database: %w", err)
	}

	j.broadcastProgress(database.ServerID, "creating", fmt.Sprintf("Creating database: %s", database.Name))

	// TODO: Run the actual database creation task
	// _, err = j.RunTask(database.Server, tasks.NewCreateDatabase(&database)).
	//     AsRoot().
	//     Dispatch(ctx)
	// if err != nil {
	//     return err
	// }

	// Mark the database as installed
	if err := j.MarkAsInstalled(j.DB, database); err != nil {
		return fmt.Errorf("failed to update database status: %w", err)
	}

	j.broadcastProgress(database.ServerID, "created", fmt.Sprintf("Database %s created successfully", database.Name))

	return nil
}

// Failed handles job failure
func (j *InstallDatabaseJob) Failed(ctx context.Context, t *asynq.Task, err error) {
	payload, unmarshalErr := jobs.UnmarshalPayload[InstallDatabasePayload](t)
	if unmarshalErr != nil {
		j.Logger.Error().Err(unmarshalErr).Msg("Failed to unmarshal payload in failure handler")
		return
	}

	j.Logger.Error().
		Err(err).
		Str("database_id", payload.DatabaseID).
		Msg("Failed to create database")

	// Fetch the database to get server ID for broadcasting
	database, findErr := j.Repo.FindDatabaseByID(ctx, payload.DatabaseID)
	if findErr != nil {
		return
	}

	// Mark installation as failed
	j.MarkInstallationFailed(j.DB, database)

	j.broadcastProgress(database.ServerID, "failed", fmt.Sprintf("Failed to create database: %s", database.Name))
}

func (j *InstallDatabaseJob) broadcastProgress(serverID, status, message string) {
	j.BroadcastToServer(serverID, "server.database.progress", map[string]interface{}{
		"server_id": serverID,
		"status":    status,
		"message":   message,
	})
}
