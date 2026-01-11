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

const TypeRestartService = "server:restart_service"

// RestartServicePayload contains data for restarting a service
type RestartServicePayload struct {
	ServerID  string               `json:"server_id"`
	Software  enums.Software       `json:"software"`
	Operation enums.ServiceOption  `json:"operation"`
}

// RestartServiceJob handles restarting a service on a server
type RestartServiceJob struct {
	db     *gorm.DB
	ws     *websocket.Hub
	logger *zerolog.Logger
}

// NewRestartServiceJob creates a new restart service job handler
func NewRestartServiceJob(db *gorm.DB, ws *websocket.Hub, logger *zerolog.Logger) *RestartServiceJob {
	return &RestartServiceJob{
		db:     db,
		ws:     ws,
		logger: logger,
	}
}

// NewRestartServiceTask creates a new asynq task for restarting a service
func NewRestartServiceTask(serverID string, software enums.Software, operation enums.ServiceOption) (*asynq.Task, error) {
	payload, err := json.Marshal(RestartServicePayload{
		ServerID:  serverID,
		Software:  software,
		Operation: operation,
	})
	if err != nil {
		return nil, err
	}

	return asynq.NewTask(TypeRestartService, payload), nil
}

// Handle processes the restart service job
func (j *RestartServiceJob) Handle(ctx context.Context, t *asynq.Task) error {
	var payload RestartServicePayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	j.logger.Info().
		Str("server_id", payload.ServerID).
		Str("software", string(payload.Software)).
		Str("operation", string(payload.Operation)).
		Msg("Performing service operation")

	// Fetch the server
	var server models.Server
	if err := j.db.First(&server, "id = ?", payload.ServerID).Error; err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	j.broadcastProgress(payload.ServerID, "operating", fmt.Sprintf("%s %s...", payload.Operation.Label(), payload.Software.Label()))

	// TODO: Implement actual service operation:
	// 1. Get the start task for the software
	// 2. Connect to server via SSH
	// 3. Execute the service command
	// 4. Update service status

	// Update service status to running
	if err := j.db.Model(&models.InstalledService{}).
		Where("server_id = ? AND software = ?", payload.ServerID, payload.Software).
		Update("status", enums.ServiceStatusRunning).Error; err != nil {
		return fmt.Errorf("failed to update service status: %w", err)
	}

	j.broadcastProgress(payload.ServerID, "running", fmt.Sprintf("%s %s completed", payload.Operation.Label(), payload.Software.Label()))

	return nil
}

// Failed handles job failure
func (j *RestartServiceJob) Failed(ctx context.Context, t *asynq.Task, err error) {
	var payload RestartServicePayload
	if unmarshalErr := json.Unmarshal(t.Payload(), &payload); unmarshalErr != nil {
		j.logger.Error().Err(unmarshalErr).Msg("Failed to unmarshal payload in failure handler")
		return
	}

	j.logger.Error().
		Err(err).
		Str("server_id", payload.ServerID).
		Str("software", string(payload.Software)).
		Str("operation", string(payload.Operation)).
		Msg("Failed to perform service operation")

	j.broadcastProgress(payload.ServerID, "failed", fmt.Sprintf("Failed to %s %s", payload.Operation.Label(), payload.Software.Label()))
}

func (j *RestartServiceJob) broadcastProgress(serverID, status, message string) {
	j.ws.BroadcastToServer(serverID, "server.service.progress", map[string]interface{}{
		"server_id": serverID,
		"status":    status,
		"message":   message,
	})
}
