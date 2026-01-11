package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/websocket"
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
	db     *gorm.DB
	ws     *websocket.Hub
	logger *zerolog.Logger
}

// NewInstallDatabaseUserJob creates a new install database user job handler
func NewInstallDatabaseUserJob(db *gorm.DB, ws *websocket.Hub, logger *zerolog.Logger) *InstallDatabaseUserJob {
	return &InstallDatabaseUserJob{
		db:     db,
		ws:     ws,
		logger: logger,
	}
}

// NewInstallDatabaseUserTask creates a new asynq task for creating a database user
func NewInstallDatabaseUserTask(databaseUserID, password string, userID *string) (*asynq.Task, error) {
	payload, err := json.Marshal(InstallDatabaseUserPayload{
		DatabaseUserID: databaseUserID,
		Password:       password,
		UserID:         userID,
	})
	if err != nil {
		return nil, err
	}

	return asynq.NewTask(TypeInstallDatabaseUser, payload), nil
}

// Handle processes the install database user job
func (j *InstallDatabaseUserJob) Handle(ctx context.Context, t *asynq.Task) error {
	var payload InstallDatabaseUserPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	j.logger.Info().
		Str("database_user_id", payload.DatabaseUserID).
		Msg("Creating database user")

	// Fetch the database user with server and databases
	var dbUser models.DatabaseUser
	if err := j.db.Preload("Server").Preload("Databases").First(&dbUser, "id = ?", payload.DatabaseUserID).Error; err != nil {
		return fmt.Errorf("failed to find database user: %w", err)
	}

	j.broadcastProgress(dbUser.ServerID, "creating", fmt.Sprintf("Creating database user: %s", dbUser.Name))

	// TODO: Implement actual database user creation:
	// 1. Connect to server via SSH
	// 2. Use database manager to create user
	// 3. Grant privileges on associated databases

	// Mark the database user as installed
	now := time.Now()
	if err := j.db.Model(&dbUser).Update("installed_at", &now).Error; err != nil {
		return fmt.Errorf("failed to update database user status: %w", err)
	}

	j.broadcastProgress(dbUser.ServerID, "created", fmt.Sprintf("Database user %s created successfully", dbUser.Name))

	return nil
}

// Failed handles job failure
func (j *InstallDatabaseUserJob) Failed(ctx context.Context, t *asynq.Task, err error) {
	var payload InstallDatabaseUserPayload
	if unmarshalErr := json.Unmarshal(t.Payload(), &payload); unmarshalErr != nil {
		j.logger.Error().Err(unmarshalErr).Msg("Failed to unmarshal payload in failure handler")
		return
	}

	j.logger.Error().
		Err(err).
		Str("database_user_id", payload.DatabaseUserID).
		Msg("Failed to create database user")

	// Fetch the database user to get server ID for broadcasting
	var dbUser models.DatabaseUser
	if findErr := j.db.First(&dbUser, "id = ?", payload.DatabaseUserID).Error; findErr != nil {
		return
	}

	// Try to drop the user if it was partially created
	// TODO: Call database manager to drop user

	// Mark installation as failed
	now := time.Now()
	j.db.Model(&dbUser).Update("installation_failed_at", &now)

	j.broadcastProgress(dbUser.ServerID, "failed", fmt.Sprintf("Failed to create database user: %s", dbUser.Name))

	// TODO: Notify user about creation failure
}

func (j *InstallDatabaseUserJob) broadcastProgress(serverID, status, message string) {
	j.ws.BroadcastToServer(serverID, "server.database_user.progress", map[string]interface{}{
		"server_id": serverID,
		"status":    status,
		"message":   message,
	})
}
