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

const TypeUninstallDatabase = "database:uninstall"

// UninstallDatabasePayload contains data for uninstalling a database
type UninstallDatabasePayload struct {
	DatabaseID string  `json:"database_id"`
	UserID     *string `json:"user_id,omitempty"`
}

// UninstallDatabaseJob handles database uninstallation from a server
type UninstallDatabaseJob struct {
	db     *gorm.DB
	ws     *websocket.Hub
	logger *zerolog.Logger
}

// NewUninstallDatabaseJob creates a new uninstall database job handler
func NewUninstallDatabaseJob(db *gorm.DB, ws *websocket.Hub, logger *zerolog.Logger) *UninstallDatabaseJob {
	return &UninstallDatabaseJob{
		db:     db,
		ws:     ws,
		logger: logger,
	}
}

// NewUninstallDatabaseTask creates a new asynq task for uninstalling a database
func NewUninstallDatabaseTask(databaseID string, userID *string) (*asynq.Task, error) {
	payload, err := json.Marshal(UninstallDatabasePayload{
		DatabaseID: databaseID,
		UserID:     userID,
	})
	if err != nil {
		return nil, err
	}

	return asynq.NewTask(TypeUninstallDatabase, payload), nil
}

// Handle processes the uninstall database job
func (j *UninstallDatabaseJob) Handle(ctx context.Context, t *asynq.Task) error {
	var payload UninstallDatabasePayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	j.logger.Info().
		Str("database_id", payload.DatabaseID).
		Msg("Uninstalling database")

	// Fetch the database with server
	var database models.Database
	if err := j.db.Preload("Server").First(&database, "id = ?", payload.DatabaseID).Error; err != nil {
		return fmt.Errorf("failed to find database: %w", err)
	}

	j.broadcastProgress(database.ServerID, payload.DatabaseID, "uninstalling", fmt.Sprintf("Dropping database: %s", database.Name))

	// TODO: Implement actual database deletion:
	// 1. Connect to server via SSH
	// 2. Determine database type (MySQL/PostgreSQL)
	// 3. Execute DROP DATABASE command
	// 4. Delete database record

	// Delete the database record
	if err := j.db.Delete(&database).Error; err != nil {
		return fmt.Errorf("failed to delete database record: %w", err)
	}

	j.broadcastProgress(database.ServerID, payload.DatabaseID, "deleted", fmt.Sprintf("Database %s deleted successfully", database.Name))

	return nil
}

// Failed handles job failure
func (j *UninstallDatabaseJob) Failed(ctx context.Context, t *asynq.Task, err error) {
	var payload UninstallDatabasePayload
	if unmarshalErr := json.Unmarshal(t.Payload(), &payload); unmarshalErr != nil {
		j.logger.Error().Err(unmarshalErr).Msg("Failed to unmarshal payload in failure handler")
		return
	}

	j.logger.Error().
		Err(err).
		Str("database_id", payload.DatabaseID).
		Msg("Failed to uninstall database")

	// Fetch the database to get server ID for broadcasting
	var database models.Database
	if findErr := j.db.First(&database, "id = ?", payload.DatabaseID).Error; findErr != nil {
		return
	}

	j.broadcastProgress(database.ServerID, payload.DatabaseID, "failed", fmt.Sprintf("Failed to delete database: %s", database.Name))
}

func (j *UninstallDatabaseJob) broadcastProgress(serverID, databaseID, status, message string) {
	j.ws.BroadcastToServer(serverID, "database.progress", map[string]interface{}{
		"server_id":   serverID,
		"database_id": databaseID,
		"status":      status,
		"message":     message,
		"timestamp":   time.Now().Format(time.RFC3339),
	})
}
