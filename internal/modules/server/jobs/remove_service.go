package jobs

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/server/enums"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/websocket"
)

const TypeRemoveService = "server:remove_service"

// RemoveServicePayload contains data for removing a service from a server
type RemoveServicePayload struct {
	ServerID  string              `json:"server_id"`
	Software  enums.Software      `json:"software"`
	Operation enums.ServiceOption `json:"operation"`
}

// RemoveServiceJob handles removing a service from a server
type RemoveServiceJob struct {
	db     *gorm.DB
	ws     *websocket.Hub
	logger *zerolog.Logger
}

// NewRemoveServiceJob creates a new remove service job handler
func NewRemoveServiceJob(db *gorm.DB, ws *websocket.Hub, logger *zerolog.Logger) *RemoveServiceJob {
	return &RemoveServiceJob{
		db:     db,
		ws:     ws,
		logger: logger,
	}
}

// NewRemoveServiceTask creates a new asynq task for removing a service
func NewRemoveServiceTask(serverID string, software enums.Software, operation enums.ServiceOption) (*asynq.Task, error) {
	payload, err := json.Marshal(RemoveServicePayload{
		ServerID:  serverID,
		Software:  software,
		Operation: operation,
	})
	if err != nil {
		return nil, err
	}

	return asynq.NewTask(TypeRemoveService, payload), nil
}

// Handle processes the remove service job
func (j *RemoveServiceJob) Handle(ctx context.Context, t *asynq.Task) error {
	var payload RemoveServicePayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	j.logger.Info().
		Str("server_id", payload.ServerID).
		Str("software", string(payload.Software)).
		Str("operation", string(payload.Operation)).
		Msg("Removing service from server")

	// Fetch the server
	var server models.Server
	if err := j.db.First(&server, "id = ?", payload.ServerID).Error; err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	j.broadcastProgress(payload.ServerID, "removing", fmt.Sprintf("Removing %s...", payload.Software.Label()))

	// TODO: Implement actual service removal:
	// 1. Get the remove task for the software
	// 2. Connect to server via SSH
	// 3. Execute the removal command
	// 4. Clean up configuration files

	// Delete the service record
	if err := j.db.Where("server_id = ? AND software = ?", payload.ServerID, payload.Software).
		Delete(&models.InstalledService{}).Error; err != nil {
		return fmt.Errorf("failed to delete service record: %w", err)
	}

	j.broadcastProgress(payload.ServerID, "removed", fmt.Sprintf("%s removed successfully", payload.Software.Label()))

	// TODO: Dispatch ServiceDeleted event

	return nil
}

// Failed handles job failure
func (j *RemoveServiceJob) Failed(ctx context.Context, t *asynq.Task, err error) {
	var payload RemoveServicePayload
	if unmarshalErr := json.Unmarshal(t.Payload(), &payload); unmarshalErr != nil {
		j.logger.Error().Err(unmarshalErr).Msg("Failed to unmarshal payload in failure handler")
		return
	}

	j.logger.Error().
		Err(err).
		Str("server_id", payload.ServerID).
		Str("software", string(payload.Software)).
		Str("operation", string(payload.Operation)).
		Msg("Failed to remove service")

	j.broadcastProgress(payload.ServerID, "failed", fmt.Sprintf("Failed to remove %s", payload.Software.Label()))
}

func (j *RemoveServiceJob) broadcastProgress(serverID, status, message string) {
	j.ws.BroadcastToServer(serverID, "server.service.progress", map[string]interface{}{
		"server_id": serverID,
		"status":    status,
		"message":   message,
	})
}
