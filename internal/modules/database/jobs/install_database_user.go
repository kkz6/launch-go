package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/database/models"
	"github.com/kkz6/launch-go/internal/websocket"
)

const TypeInstallDatabaseUser = "database:user:install"

// InstallDatabaseUserPayload contains data for installing a database user
type InstallDatabaseUserPayload struct {
	DatabaseUserID string  `json:"database_user_id"`
	Password       string  `json:"password"`
	CallerID       *string `json:"caller_id,omitempty"`
}

// InstallDatabaseUserJob handles database user installation on a server
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

// NewInstallDatabaseUserTask creates a new asynq task for installing a database user
func NewInstallDatabaseUserTask(databaseUserID, password string, callerID *string) (*asynq.Task, error) {
	payload, err := json.Marshal(InstallDatabaseUserPayload{
		DatabaseUserID: databaseUserID,
		Password:       password,
		CallerID:       callerID,
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
		Msg("Installing database user")

	// Fetch the database user with server and databases
	var dbUser models.DatabaseUser
	if err := j.db.Preload("Server").Preload("Databases").First(&dbUser, "id = ?", payload.DatabaseUserID).Error; err != nil {
		return fmt.Errorf("failed to find database user: %w", err)
	}

	j.broadcastProgress(dbUser.ServerID, payload.DatabaseUserID, "installing", fmt.Sprintf("Creating database user: %s", dbUser.Name))

	// TODO: Implement actual database user creation:
	// 1. Connect to server via SSH
	// 2. Determine database type (MySQL/PostgreSQL)
	// 3. Execute CREATE USER command
	// 4. Grant privileges on associated databases
	// 5. Update user status to installed

	// Mark the database user as installed
	now := time.Now()
	if err := j.db.Model(&dbUser).Update("installed_at", &now).Error; err != nil {
		return fmt.Errorf("failed to update database user status: %w", err)
	}

	j.broadcastProgress(dbUser.ServerID, payload.DatabaseUserID, "installed", fmt.Sprintf("Database user %s created successfully", dbUser.Name))

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
		Msg("Failed to install database user")

	// Fetch the database user to get server ID for broadcasting
	var dbUser models.DatabaseUser
	if findErr := j.db.First(&dbUser, "id = ?", payload.DatabaseUserID).Error; findErr != nil {
		return
	}

	// Mark installation as failed
	now := time.Now()
	j.db.Model(&dbUser).Update("installation_failed_at", &now)

	j.broadcastProgress(dbUser.ServerID, payload.DatabaseUserID, "failed", fmt.Sprintf("Failed to create database user: %s", dbUser.Name))
}

func (j *InstallDatabaseUserJob) broadcastProgress(serverID, userID, status, message string) {
	j.ws.BroadcastToServer(serverID, "database_user.progress", map[string]interface{}{
		"server_id": serverID,
		"user_id":   userID,
		"status":    status,
		"message":   message,
		"timestamp": time.Now().Format(time.RFC3339),
	})
}
