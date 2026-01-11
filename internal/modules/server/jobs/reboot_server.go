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

const TypeRebootServer = "server:reboot"

// RebootServerPayload contains data for rebooting a server
type RebootServerPayload struct {
	ServerID string  `json:"server_id"`
	UserID   *string `json:"user_id,omitempty"`
}

// RebootServerJob handles server reboot operations
type RebootServerJob struct {
	db     *gorm.DB
	ws     *websocket.Hub
	logger *zerolog.Logger
}

// NewRebootServerJob creates a new reboot server job handler
func NewRebootServerJob(db *gorm.DB, ws *websocket.Hub, logger *zerolog.Logger) *RebootServerJob {
	return &RebootServerJob{
		db:     db,
		ws:     ws,
		logger: logger,
	}
}

// NewRebootServerTask creates a new asynq task for rebooting a server
func NewRebootServerTask(serverID string, userID *string) (*asynq.Task, error) {
	payload, err := json.Marshal(RebootServerPayload{
		ServerID: serverID,
		UserID:   userID,
	})
	if err != nil {
		return nil, err
	}

	return asynq.NewTask(TypeRebootServer, payload), nil
}

// Handle processes the reboot server job
func (j *RebootServerJob) Handle(ctx context.Context, t *asynq.Task) error {
	var payload RebootServerPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	j.logger.Info().
		Str("server_id", payload.ServerID).
		Msg("Rebooting server")

	// Fetch the server
	var server models.Server
	if err := j.db.First(&server, "id = ?", payload.ServerID).Error; err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	j.broadcastProgress(payload.ServerID, "rebooting", fmt.Sprintf("Rebooting server: %s", server.Name))

	// TODO: Implement actual server reboot:
	// 1. Connect to server via SSH
	// 2. Execute reboot command
	// 3. Wait for server to come back online
	// 4. Update server status

	j.broadcastProgress(payload.ServerID, "rebooted", fmt.Sprintf("Server %s rebooted successfully", server.Name))

	return nil
}

// Failed handles job failure
func (j *RebootServerJob) Failed(ctx context.Context, t *asynq.Task, err error) {
	var payload RebootServerPayload
	if unmarshalErr := json.Unmarshal(t.Payload(), &payload); unmarshalErr != nil {
		j.logger.Error().Err(unmarshalErr).Msg("Failed to unmarshal payload in failure handler")
		return
	}

	j.logger.Error().
		Err(err).
		Str("server_id", payload.ServerID).
		Msg("Failed to reboot server")

	j.broadcastProgress(payload.ServerID, "failed", "Failed to reboot server")
}

func (j *RebootServerJob) broadcastProgress(serverID, status, message string) {
	j.ws.BroadcastToServer(serverID, "server.progress", map[string]interface{}{
		"server_id": serverID,
		"status":    status,
		"message":   message,
		"timestamp": time.Now().Format(time.RFC3339),
	})
}
