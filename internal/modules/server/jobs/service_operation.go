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

const TypeServiceOperation = "server:service_operation"

// ServiceOperationPayload contains data for service operations
type ServiceOperationPayload struct {
	ServerID  string  `json:"server_id"`
	ServiceID string  `json:"service_id"`
	Operation string  `json:"operation"` // restart, stop, remove, status
	UserID    *string `json:"user_id,omitempty"`
}

// ServiceOperationJob handles service operation tasks
type ServiceOperationJob struct {
	db     *gorm.DB
	ws     *websocket.Hub
	logger *zerolog.Logger
}

// NewServiceOperationJob creates a new service operation job handler
func NewServiceOperationJob(db *gorm.DB, ws *websocket.Hub, logger *zerolog.Logger) *ServiceOperationJob {
	return &ServiceOperationJob{
		db:     db,
		ws:     ws,
		logger: logger,
	}
}

// NewServiceOperationTask creates a new asynq task for service operations
func NewServiceOperationTask(serverID, serviceID, operation string, userID *string) (*asynq.Task, error) {
	payload, err := json.Marshal(ServiceOperationPayload{
		ServerID:  serverID,
		ServiceID: serviceID,
		Operation: operation,
		UserID:    userID,
	})
	if err != nil {
		return nil, err
	}

	return asynq.NewTask(TypeServiceOperation, payload), nil
}

// Handle processes the service operation job
func (j *ServiceOperationJob) Handle(ctx context.Context, t *asynq.Task) error {
	var payload ServiceOperationPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	j.logger.Info().
		Str("server_id", payload.ServerID).
		Str("service_id", payload.ServiceID).
		Str("operation", payload.Operation).
		Msg("Processing service operation")

	// Fetch the server
	var server models.Server
	if err := j.db.First(&server, "id = ?", payload.ServerID).Error; err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	// Fetch the installed service
	var service models.InstalledService
	if err := j.db.First(&service, "id = ?", payload.ServiceID).Error; err != nil {
		return fmt.Errorf("failed to find service: %w", err)
	}

	j.broadcastProgress(payload.ServerID, payload.ServiceID, "processing",
		fmt.Sprintf("Processing %s operation for service: %s", payload.Operation, service.Software))

	// TODO: Implement actual service operations:
	// 1. Connect to server via SSH
	// 2. Execute appropriate systemctl command based on operation
	// 3. Update service status in database

	switch payload.Operation {
	case "restart":
		// TODO: systemctl restart <service>
	case "stop":
		// TODO: systemctl stop <service>
	case "remove":
		// TODO: systemctl stop <service> && systemctl disable <service>
		if err := j.db.Delete(&service).Error; err != nil {
			return fmt.Errorf("failed to delete service record: %w", err)
		}
	case "status":
		// TODO: systemctl status <service>
	default:
		return fmt.Errorf("unknown operation: %s", payload.Operation)
	}

	j.broadcastProgress(payload.ServerID, payload.ServiceID, "completed",
		fmt.Sprintf("Service %s %s operation completed", service.Software, payload.Operation))

	return nil
}

// Failed handles job failure
func (j *ServiceOperationJob) Failed(ctx context.Context, t *asynq.Task, err error) {
	var payload ServiceOperationPayload
	if unmarshalErr := json.Unmarshal(t.Payload(), &payload); unmarshalErr != nil {
		j.logger.Error().Err(unmarshalErr).Msg("Failed to unmarshal payload in failure handler")
		return
	}

	j.logger.Error().
		Err(err).
		Str("server_id", payload.ServerID).
		Str("service_id", payload.ServiceID).
		Str("operation", payload.Operation).
		Msg("Failed to process service operation")

	j.broadcastProgress(payload.ServerID, payload.ServiceID, "failed",
		fmt.Sprintf("Failed to %s service", payload.Operation))
}

func (j *ServiceOperationJob) broadcastProgress(serverID, serviceID, status, message string) {
	j.ws.BroadcastToServer(serverID, "service.progress", map[string]interface{}{
		"server_id":  serverID,
		"service_id": serviceID,
		"status":     status,
		"message":    message,
		"timestamp":  time.Now().Format(time.RFC3339),
	})
}
