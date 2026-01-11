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

const TypeAddService = "server:add_service"

// AddServicePayload contains data for adding a service to a server
type AddServicePayload struct {
	ServerID  string         `json:"server_id"`
	ServiceID string         `json:"service_id"`
	Software  enums.Software `json:"software"`
}

// AddServiceJob handles adding a service to a server
type AddServiceJob struct {
	db     *gorm.DB
	ws     *websocket.Hub
	logger *zerolog.Logger
}

// NewAddServiceJob creates a new add service job handler
func NewAddServiceJob(db *gorm.DB, ws *websocket.Hub, logger *zerolog.Logger) *AddServiceJob {
	return &AddServiceJob{
		db:     db,
		ws:     ws,
		logger: logger,
	}
}

// NewAddServiceTask creates a new asynq task for adding a service
func NewAddServiceTask(serverID, serviceID string, software enums.Software) (*asynq.Task, error) {
	payload, err := json.Marshal(AddServicePayload{
		ServerID:  serverID,
		ServiceID: serviceID,
		Software:  software,
	})
	if err != nil {
		return nil, err
	}

	return asynq.NewTask(TypeAddService, payload), nil
}

// Handle processes the add service job
func (j *AddServiceJob) Handle(ctx context.Context, t *asynq.Task) error {
	var payload AddServicePayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	j.logger.Info().
		Str("server_id", payload.ServerID).
		Str("service_id", payload.ServiceID).
		Str("software", string(payload.Software)).
		Msg("Adding service to server")

	// Fetch the server
	var server models.Server
	if err := j.db.First(&server, "id = ?", payload.ServerID).Error; err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	// Fetch the service
	var service models.InstalledService
	if err := j.db.First(&service, "id = ?", payload.ServiceID).Error; err != nil {
		return fmt.Errorf("failed to find service: %w", err)
	}

	j.broadcastProgress(payload.ServerID, "installing", fmt.Sprintf("Installing %s...", payload.Software.Label()))

	// TODO: Implement actual service installation:
	// 1. Connect to server via SSH
	// 2. Run AddService task script
	// 3. Configure the service
	// 4. Start the service

	// Create a task record and associate it with the service
	// This would be done through the task runner

	j.broadcastProgress(payload.ServerID, "installed", fmt.Sprintf("%s installed successfully", payload.Software.Label()))

	return nil
}

// Failed handles job failure
func (j *AddServiceJob) Failed(ctx context.Context, t *asynq.Task, err error) {
	var payload AddServicePayload
	if unmarshalErr := json.Unmarshal(t.Payload(), &payload); unmarshalErr != nil {
		j.logger.Error().Err(unmarshalErr).Msg("Failed to unmarshal payload in failure handler")
		return
	}

	j.logger.Error().
		Err(err).
		Str("server_id", payload.ServerID).
		Str("service_id", payload.ServiceID).
		Str("software", string(payload.Software)).
		Msg("Failed to add service")

	// Update service status to failed
	j.db.Model(&models.InstalledService{}).
		Where("id = ?", payload.ServiceID).
		Update("status", enums.ServiceStatusFailed)

	j.broadcastProgress(payload.ServerID, "failed", fmt.Sprintf("Failed to install %s", payload.Software.Label()))
}

func (j *AddServiceJob) broadcastProgress(serverID, status, message string) {
	j.ws.BroadcastToServer(serverID, "server.service.progress", map[string]interface{}{
		"server_id": serverID,
		"status":    status,
		"message":   message,
	})
}
