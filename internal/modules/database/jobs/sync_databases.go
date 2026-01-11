package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/websocket"
)

const TypeSyncDatabases = "database:sync"

// SyncDatabasesPayload contains data for syncing databases from a server
type SyncDatabasesPayload struct {
	ServerID string  `json:"server_id"`
	UserID   *string `json:"user_id,omitempty"`
}

// SyncDatabasesJob handles syncing databases from a server
type SyncDatabasesJob struct {
	db     *gorm.DB
	ws     *websocket.Hub
	logger *zerolog.Logger
}

// NewSyncDatabasesJob creates a new sync databases job handler
func NewSyncDatabasesJob(db *gorm.DB, ws *websocket.Hub, logger *zerolog.Logger) *SyncDatabasesJob {
	return &SyncDatabasesJob{
		db:     db,
		ws:     ws,
		logger: logger,
	}
}

// NewSyncDatabasesTask creates a new asynq task for syncing databases
func NewSyncDatabasesTask(serverID string, userID *string) (*asynq.Task, error) {
	payload, err := json.Marshal(SyncDatabasesPayload{
		ServerID: serverID,
		UserID:   userID,
	})
	if err != nil {
		return nil, err
	}

	return asynq.NewTask(TypeSyncDatabases, payload), nil
}

// Handle processes the sync databases job
func (j *SyncDatabasesJob) Handle(ctx context.Context, t *asynq.Task) error {
	var payload SyncDatabasesPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	j.logger.Info().
		Str("server_id", payload.ServerID).
		Msg("Syncing databases from server")

	j.broadcastProgress(payload.ServerID, "syncing", "Syncing databases from server...")

	// TODO: Implement actual database sync:
	// 1. Connect to server via SSH
	// 2. Determine database type (MySQL/PostgreSQL)
	// 3. Execute SHOW DATABASES command
	// 4. Compare with existing records in database
	// 5. Create records for new databases
	// 6. Mark missing databases as uninstalled

	j.broadcastProgress(payload.ServerID, "synced", "Database sync completed")

	return nil
}

// Failed handles job failure
func (j *SyncDatabasesJob) Failed(ctx context.Context, t *asynq.Task, err error) {
	var payload SyncDatabasesPayload
	if unmarshalErr := json.Unmarshal(t.Payload(), &payload); unmarshalErr != nil {
		j.logger.Error().Err(unmarshalErr).Msg("Failed to unmarshal payload in failure handler")
		return
	}

	j.logger.Error().
		Err(err).
		Str("server_id", payload.ServerID).
		Msg("Failed to sync databases")

	j.broadcastProgress(payload.ServerID, "failed", "Failed to sync databases from server")
}

func (j *SyncDatabasesJob) broadcastProgress(serverID, status, message string) {
	j.ws.BroadcastToServer(serverID, "database.sync.progress", map[string]interface{}{
		"server_id": serverID,
		"status":    status,
		"message":   message,
		"timestamp": time.Now().Format(time.RFC3339),
	})
}
