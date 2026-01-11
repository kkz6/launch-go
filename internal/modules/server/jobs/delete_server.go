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

const TypeDeleteServer = "server:delete"

// DeleteServerPayload contains data for deleting a server
type DeleteServerPayload struct {
	ServerID string  `json:"server_id"`
	TeamID   string  `json:"team_id"`
	UserID   *string `json:"user_id,omitempty"`
}

// DeleteServerJob handles server deletion operations
type DeleteServerJob struct {
	db     *gorm.DB
	ws     *websocket.Hub
	logger *zerolog.Logger
}

// NewDeleteServerJob creates a new delete server job handler
func NewDeleteServerJob(db *gorm.DB, ws *websocket.Hub, logger *zerolog.Logger) *DeleteServerJob {
	return &DeleteServerJob{
		db:     db,
		ws:     ws,
		logger: logger,
	}
}

// NewDeleteServerTask creates a new asynq task for deleting a server
func NewDeleteServerTask(serverID, teamID string, userID *string) (*asynq.Task, error) {
	payload, err := json.Marshal(DeleteServerPayload{
		ServerID: serverID,
		TeamID:   teamID,
		UserID:   userID,
	})
	if err != nil {
		return nil, err
	}

	return asynq.NewTask(TypeDeleteServer, payload), nil
}

// Handle processes the delete server job
func (j *DeleteServerJob) Handle(ctx context.Context, t *asynq.Task) error {
	var payload DeleteServerPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	j.logger.Info().
		Str("server_id", payload.ServerID).
		Str("team_id", payload.TeamID).
		Msg("Deleting server")

	// Fetch the server
	var server models.Server
	if err := j.db.First(&server, "id = ?", payload.ServerID).Error; err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	j.broadcastProgress(payload.ServerID, "deleting", fmt.Sprintf("Deleting server: %s", server.Name))

	// TODO: Implement actual server deletion:
	// 1. Delete server from cloud provider (if managed)
	// 2. Clean up associated resources (sites, databases, etc.)
	// 3. Delete server record from database

	// Delete the server record
	if err := j.db.Delete(&server).Error; err != nil {
		return fmt.Errorf("failed to delete server record: %w", err)
	}

	j.broadcastProgress(payload.ServerID, "deleted", fmt.Sprintf("Server %s deleted successfully", server.Name))

	return nil
}

// Failed handles job failure
func (j *DeleteServerJob) Failed(ctx context.Context, t *asynq.Task, err error) {
	var payload DeleteServerPayload
	if unmarshalErr := json.Unmarshal(t.Payload(), &payload); unmarshalErr != nil {
		j.logger.Error().Err(unmarshalErr).Msg("Failed to unmarshal payload in failure handler")
		return
	}

	j.logger.Error().
		Err(err).
		Str("server_id", payload.ServerID).
		Msg("Failed to delete server")

	j.broadcastProgress(payload.ServerID, "failed", "Failed to delete server")
}

func (j *DeleteServerJob) broadcastProgress(serverID, status, message string) {
	j.ws.BroadcastToServer(serverID, "server.progress", map[string]interface{}{
		"server_id": serverID,
		"status":    status,
		"message":   message,
		"timestamp": time.Now().Format(time.RFC3339),
	})
}
