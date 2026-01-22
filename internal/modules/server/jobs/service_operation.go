package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

const TypeServiceOperation = "server:service_operation"

type ServiceOperationPayload struct {
	ServerID  string  `json:"server_id"`
	ServiceID string  `json:"service_id"`
	Operation string  `json:"operation"`
	UserID    *string `json:"user_id,omitempty"`
}

// ServiceOperationJob performs an operation (start/stop/restart/reload) on a service.
// Similar to Laravel's Modules\Server\Jobs\ServiceOperation
type ServiceOperationJob struct {
	Deps    *JobDeps
	Payload ServiceOperationPayload

	server  *models.Server
	service *models.InstalledService
}

func NewServiceOperationJob(p ServiceOperationPayload) pkgjobs.Handler {
	return &ServiceOperationJob{Deps: deps, Payload: p}
}

// Handle processes the job
func (j *ServiceOperationJob) Handle(ctx context.Context) error {
	var err error

	// Find the service
	j.service, err = j.Deps.Repos.Service().FindByID(ctx, j.Payload.ServiceID)
	if err != nil {
		return fmt.Errorf("failed to find service: %w", err)
	}

	// Find the server
	j.server, err = j.Deps.Repos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	// Get the service name for systemd
	serviceName := j.service.GetServiceName()

	// Create the appropriate task based on operation
	var task taskrunner.Task

	switch j.Payload.Operation {
	case "start":
		task = tasks.StartService(serviceName)
	case "stop":
		task = tasks.StopService(serviceName)
	case "restart":
		task = tasks.RestartService(serviceName)
	case "reload":
		task = tasks.ReloadService(serviceName)
	default:
		return fmt.Errorf("unknown operation: %s", j.Payload.Operation)
	}

	result, err := j.Deps.RunTask(j.server, task).
		AsRoot().
		Dispatch(ctx)

	if err != nil {
		return fmt.Errorf("failed to %s service: %w", j.Payload.Operation, err)
	}

	if !result.IsSuccessful() {
		return fmt.Errorf("failed to %s service: %s", j.Payload.Operation, result.GetOutput())
	}

	j.Deps.Logger.Info().
		Str("service_id", j.service.ID).
		Str("server_id", j.server.ID).
		Str("operation", j.Payload.Operation).
		Msg("service operation completed")

	// Broadcast event
	j.Deps.BroadcastServerEvent(j.server, "service.operation", map[string]any{
		"service_id": j.service.ID,
		"server_id":  j.server.ID,
		"operation":  j.Payload.Operation,
	})

	return nil
}

// Failed is called when the job fails after all retries
func (j *ServiceOperationJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).
		Str("service_id", j.Payload.ServiceID).
		Str("server_id", j.Payload.ServerID).
		Str("operation", j.Payload.Operation).
		Msg("failed to perform service operation")
}

// NewServiceOperationTask creates an asynq task for performing a service operation
func NewServiceOperationTask(serverID, serviceID, operation string, userID *string) (*asynq.Task, error) {
	return pkgjobs.Task(TypeServiceOperation, ServiceOperationPayload{
		ServerID:  serverID,
		ServiceID: serviceID,
		Operation: operation,
		UserID:    userID,
	})
}
