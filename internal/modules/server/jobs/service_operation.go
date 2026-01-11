package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/pkg/jobs"
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
	*JobContext
}

// NewServiceOperationTask creates a new asynq task for service operations
func NewServiceOperationTask(serverID, serviceID, operation string, userID *string) (*asynq.Task, error) {
	return jobs.NewTask(TypeServiceOperation, ServiceOperationPayload{
		ServerID:  serverID,
		ServiceID: serviceID,
		Operation: operation,
		UserID:    userID,
	})
}

// Handle processes the service operation job
func (j *ServiceOperationJob) Handle(ctx context.Context, t *asynq.Task) error {
	payload, err := jobs.UnmarshalPayload[ServiceOperationPayload](t)
	if err != nil {
		return err
	}

	j.Logger.Info().
		Str("server_id", payload.ServerID).
		Str("service_id", payload.ServiceID).
		Str("operation", payload.Operation).
		Msg("Processing service operation")

	// Fetch the server
	server, err := j.FindServer(ctx, payload.ServerID)
	if err != nil {
		return err
	}

	// Fetch the installed service
	var service models.InstalledService
	if err := j.DB.First(&service, "id = ?", payload.ServiceID).Error; err != nil {
		return fmt.Errorf("failed to find service: %w", err)
	}

	j.broadcastProgress(payload.ServerID, payload.ServiceID, "processing",
		fmt.Sprintf("Processing %s operation for service: %s", payload.Operation, service.Software))

	// TODO: Run the actual service operation task
	// _, err = j.RunTask(server, tasks.NewServiceOperation(&service, payload.Operation)).
	//     AsRoot().
	//     Dispatch(ctx)
	// if err != nil {
	//     return err
	// }

	_ = server // use server when implementing task execution

	switch payload.Operation {
	case "restart":
		// TODO: implement restart
	case "stop":
		// TODO: implement stop
	case "remove":
		if err := j.DB.Delete(&service).Error; err != nil {
			return fmt.Errorf("failed to delete service record: %w", err)
		}
	case "status":
		// TODO: implement status check
	default:
		return fmt.Errorf("unknown operation: %s", payload.Operation)
	}

	j.broadcastProgress(payload.ServerID, payload.ServiceID, "completed",
		fmt.Sprintf("Service %s %s operation completed", service.Software, payload.Operation))

	return nil
}

// Failed handles job failure
func (j *ServiceOperationJob) Failed(ctx context.Context, t *asynq.Task, err error) {
	payload, unmarshalErr := jobs.UnmarshalPayload[ServiceOperationPayload](t)
	if unmarshalErr != nil {
		j.Logger.Error().Err(unmarshalErr).Msg("Failed to unmarshal payload in failure handler")
		return
	}

	j.Logger.Error().
		Err(err).
		Str("server_id", payload.ServerID).
		Str("service_id", payload.ServiceID).
		Str("operation", payload.Operation).
		Msg("Failed to process service operation")

	j.broadcastProgress(payload.ServerID, payload.ServiceID, "failed",
		fmt.Sprintf("Failed to %s service", payload.Operation))
}

func (j *ServiceOperationJob) broadcastProgress(serverID, serviceID, status, message string) {
	j.BroadcastToServer(serverID, "service.progress", map[string]interface{}{
		"server_id":  serverID,
		"service_id": serviceID,
		"status":     status,
		"message":    message,
		"timestamp":  time.Now().Format(time.RFC3339),
	})
}
