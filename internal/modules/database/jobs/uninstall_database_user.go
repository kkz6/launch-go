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

const TypeUninstallDatabaseUser = "database:user:uninstall"

// UninstallDatabaseUserPayload contains data for uninstalling a database user
type UninstallDatabaseUserPayload struct {
	DatabaseUserID string  `json:"database_user_id"`
	CallerID       *string `json:"caller_id,omitempty"`
}

// UninstallDatabaseUserJob handles database user uninstallation from a server
type UninstallDatabaseUserJob struct {
	db     *gorm.DB
	ws     *websocket.Hub
	logger *zerolog.Logger
}

// NewUninstallDatabaseUserJob creates a new uninstall database user job handler
func NewUninstallDatabaseUserJob(db *gorm.DB, ws *websocket.Hub, logger *zerolog.Logger) *UninstallDatabaseUserJob {
	return &UninstallDatabaseUserJob{
		db:     db,
		ws:     ws,
		logger: logger,
	}
}

// NewUninstallDatabaseUserTask creates a new asynq task for uninstalling a database user
func NewUninstallDatabaseUserTask(databaseUserID string, callerID *string) (*asynq.Task, error) {
	payload, err := json.Marshal(UninstallDatabaseUserPayload{
		DatabaseUserID: databaseUserID,
		CallerID:       callerID,
	})
	if err != nil {
		return nil, err
	}

	return asynq.NewTask(TypeUninstallDatabaseUser, payload), nil
}

// Handle processes the uninstall database user job
func (j *UninstallDatabaseUserJob) Handle(ctx context.Context, t *asynq.Task) error {
	var payload UninstallDatabaseUserPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	j.logger.Info().
		Str("database_user_id", payload.DatabaseUserID).
		Msg("Uninstalling database user")

	// Fetch the database user with server
	var dbUser models.DatabaseUser
	if err := j.db.Preload("Server").First(&dbUser, "id = ?", payload.DatabaseUserID).Error; err != nil {
		return fmt.Errorf("failed to find database user: %w", err)
	}

	j.broadcastProgress(dbUser.ServerID, payload.DatabaseUserID, "uninstalling", fmt.Sprintf("Dropping database user: %s", dbUser.Name))

	// TODO: Implement actual database user deletion:
	// 1. Connect to server via SSH
	// 2. Determine database type (MySQL/PostgreSQL)
	// 3. Execute DROP USER command
	// 4. Delete user record

	// Delete the database user record
	if err := j.db.Delete(&dbUser).Error; err != nil {
		return fmt.Errorf("failed to delete database user record: %w", err)
	}

	j.broadcastProgress(dbUser.ServerID, payload.DatabaseUserID, "deleted", fmt.Sprintf("Database user %s deleted successfully", dbUser.Name))

	return nil
}

// Failed handles job failure
func (j *UninstallDatabaseUserJob) Failed(ctx context.Context, t *asynq.Task, err error) {
	var payload UninstallDatabaseUserPayload
	if unmarshalErr := json.Unmarshal(t.Payload(), &payload); unmarshalErr != nil {
		j.logger.Error().Err(unmarshalErr).Msg("Failed to unmarshal payload in failure handler")
		return
	}

	j.logger.Error().
		Err(err).
		Str("database_user_id", payload.DatabaseUserID).
		Msg("Failed to uninstall database user")

	// Fetch the database user to get server ID for broadcasting
	var dbUser models.DatabaseUser
	if findErr := j.db.First(&dbUser, "id = ?", payload.DatabaseUserID).Error; findErr != nil {
		return
	}

	j.broadcastProgress(dbUser.ServerID, payload.DatabaseUserID, "failed", fmt.Sprintf("Failed to delete database user: %s", dbUser.Name))
}

func (j *UninstallDatabaseUserJob) broadcastProgress(serverID, userID, status, message string) {
	j.ws.BroadcastToServer(serverID, "database_user.progress", map[string]interface{}{
		"server_id": serverID,
		"user_id":   userID,
		"status":    status,
		"message":   message,
		"timestamp": time.Now().Format(time.RFC3339),
	})
}
