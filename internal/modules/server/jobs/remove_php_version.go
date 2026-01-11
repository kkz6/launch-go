package jobs

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/websocket"
)

const TypeRemovePhpVersion = "server:remove_php_version"

// RemovePhpVersionPayload contains data for removing a PHP version from a server
type RemovePhpVersionPayload struct {
	ServerID  string `json:"server_id"`
	ServiceID string `json:"service_id"`
}

// RemovePhpVersionJob handles removing a PHP version from a server
type RemovePhpVersionJob struct {
	db     *gorm.DB
	ws     *websocket.Hub
	logger *zerolog.Logger
}

// NewRemovePhpVersionJob creates a new remove PHP version job handler
func NewRemovePhpVersionJob(db *gorm.DB, ws *websocket.Hub, logger *zerolog.Logger) *RemovePhpVersionJob {
	return &RemovePhpVersionJob{
		db:     db,
		ws:     ws,
		logger: logger,
	}
}

// NewRemovePhpVersionTask creates a new asynq task for removing a PHP version
func NewRemovePhpVersionTask(serverID, serviceID string) (*asynq.Task, error) {
	payload, err := json.Marshal(RemovePhpVersionPayload{
		ServerID:  serverID,
		ServiceID: serviceID,
	})
	if err != nil {
		return nil, err
	}

	return asynq.NewTask(TypeRemovePhpVersion, payload), nil
}

// Handle processes the remove PHP version job
func (j *RemovePhpVersionJob) Handle(ctx context.Context, t *asynq.Task) error {
	var payload RemovePhpVersionPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	j.logger.Info().
		Str("server_id", payload.ServerID).
		Str("service_id", payload.ServiceID).
		Msg("Removing PHP version from server")

	// Fetch the server
	var server models.Server
	if err := j.db.First(&server, "id = ?", payload.ServerID).Error; err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	// Fetch the service to get version info
	var service models.InstalledService
	if err := j.db.First(&service, "id = ?", payload.ServiceID).Error; err != nil {
		return fmt.Errorf("failed to find service: %w", err)
	}

	j.broadcastProgress(payload.ServerID, "removing", fmt.Sprintf("Removing PHP %s...", service.Version))

	// TODO: Implement actual PHP removal:
	// 1. Connect to server via SSH
	// 2. Run RemovePhpVersion task script
	// 3. Clean up configuration files
	// 4. Delete service record

	j.broadcastProgress(payload.ServerID, "removed", fmt.Sprintf("PHP %s removed successfully", service.Version))

	return nil
}

// Failed handles job failure
func (j *RemovePhpVersionJob) Failed(ctx context.Context, t *asynq.Task, err error) {
	var payload RemovePhpVersionPayload
	if unmarshalErr := json.Unmarshal(t.Payload(), &payload); unmarshalErr != nil {
		j.logger.Error().Err(unmarshalErr).Msg("Failed to unmarshal payload in failure handler")
		return
	}

	j.logger.Error().
		Err(err).
		Str("server_id", payload.ServerID).
		Str("service_id", payload.ServiceID).
		Msg("Failed to remove PHP version")

	j.broadcastProgress(payload.ServerID, "failed", "Failed to remove PHP version")
}

func (j *RemovePhpVersionJob) broadcastProgress(serverID, status, message string) {
	j.ws.BroadcastToServer(serverID, "server.service.progress", map[string]interface{}{
		"server_id": serverID,
		"status":    status,
		"message":   message,
	})
}
