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

const TypeInstallDatabase = "server:install_database"

// InstallDatabasePayload contains data for creating a database
type InstallDatabasePayload struct {
	DatabaseID string  `json:"database_id"`
	UserID     *string `json:"user_id,omitempty"`
}

// InstallDatabaseJob handles creating a database on a server
type InstallDatabaseJob struct {
	db     *gorm.DB
	ws     *websocket.Hub
	logger *zerolog.Logger
}

// NewInstallDatabaseJob creates a new install database job handler
func NewInstallDatabaseJob(db *gorm.DB, ws *websocket.Hub, logger *zerolog.Logger) *InstallDatabaseJob {
	return &InstallDatabaseJob{
		db:     db,
		ws:     ws,
		logger: logger,
	}
}

// NewInstallDatabaseTask creates a new asynq task for creating a database
func NewInstallDatabaseTask(databaseID string, userID *string) (*asynq.Task, error) {
	payload, err := json.Marshal(InstallDatabasePayload{
		DatabaseID: databaseID,
		UserID:     userID,
	})
	if err != nil {
		return nil, err
	}

	return asynq.NewTask(TypeInstallDatabase, payload), nil
}

// Handle processes the install database job
func (j *InstallDatabaseJob) Handle(ctx context.Context, t *asynq.Task) error {
	var payload InstallDatabasePayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	j.logger.Info().
		Str("database_id", payload.DatabaseID).
		Msg("Creating database")

	// Fetch the database with server
	var database models.Database
	if err := j.db.Preload("Server").First(&database, "id = ?", payload.DatabaseID).Error; err != nil {
		return fmt.Errorf("failed to find database: %w", err)
	}

	j.broadcastProgress(database.ServerID, "creating", fmt.Sprintf("Creating database: %s", database.Name))

	// TODO: Implement actual database creation:
	// 1. Connect to server via SSH
	// 2. Use database manager to create database
	// 3. Execute CREATE DATABASE command

	// Mark the database as installed
	now := time.Now()
	if err := j.db.Model(&database).Update("installed_at", &now).Error; err != nil {
		return fmt.Errorf("failed to update database status: %w", err)
	}

	j.broadcastProgress(database.ServerID, "created", fmt.Sprintf("Database %s created successfully", database.Name))

	return nil
}

// Failed handles job failure
func (j *InstallDatabaseJob) Failed(ctx context.Context, t *asynq.Task, err error) {
	var payload InstallDatabasePayload
	if unmarshalErr := json.Unmarshal(t.Payload(), &payload); unmarshalErr != nil {
		j.logger.Error().Err(unmarshalErr).Msg("Failed to unmarshal payload in failure handler")
		return
	}

	j.logger.Error().
		Err(err).
		Str("database_id", payload.DatabaseID).
		Msg("Failed to create database")

	// Fetch the database to get server ID for broadcasting
	var database models.Database
	if findErr := j.db.First(&database, "id = ?", payload.DatabaseID).Error; findErr != nil {
		return
	}

	// Try to drop the database if it was partially created
	// TODO: Call database manager to drop database

	// Mark installation as failed
	now := time.Now()
	j.db.Model(&database).Update("installation_failed_at", &now)

	j.broadcastProgress(database.ServerID, "failed", fmt.Sprintf("Failed to create database: %s", database.Name))

	// TODO: Notify user about creation failure
}

func (j *InstallDatabaseJob) broadcastProgress(serverID, status, message string) {
	j.ws.BroadcastToServer(serverID, "server.database.progress", map[string]interface{}{
		"server_id": serverID,
		"status":    status,
		"message":   message,
	})
}
