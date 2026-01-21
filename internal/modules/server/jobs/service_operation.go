package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

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
	pkgjobs.BaseJob[*JobContext, ServiceOperationPayload]
}

// Handle processes the job
func (j *ServiceOperationJob) Handle(ctx context.Context) error {
	// Find the service
	service, err := j.Ctx.Repos.Service().FindByID(ctx, j.Payload.ServiceID)
	if err != nil {
		return fmt.Errorf("failed to find service: %w", err)
	}

	// Find the server
	server, err := j.Ctx.Repos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	// Get the service name for systemd
	serviceName := service.GetServiceName()

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

	result, err := j.Ctx.ForServer(server).RunTask(task).
		AsRoot().
		Dispatch(ctx)

	if err != nil {
		return fmt.Errorf("failed to %s service: %w", j.Payload.Operation, err)
	}

	if !result.IsSuccessful() {
		return fmt.Errorf("failed to %s service: %s", j.Payload.Operation, result.GetOutput())
	}

	j.Ctx.LogInfo("Service operation completed",
		"service_id", service.ID,
		"server_id", server.ID,
		"operation", j.Payload.Operation,
	)

	// Broadcast event
	j.Ctx.BroadcastServerEvent(server, "service.operation", map[string]any{
		"service_id": service.ID,
		"server_id":  server.ID,
		"operation":  j.Payload.Operation,
	})

	return nil
}

// Failed is called when the job fails after all retries
func (j *ServiceOperationJob) Failed(ctx context.Context, err error) {
	j.Ctx.LogError(err, "Failed to perform service operation",
		"service_id", j.Payload.ServiceID,
		"server_id", j.Payload.ServerID,
		"operation", j.Payload.Operation,
	)
}

// NewServiceOperationJob creates a new ServiceOperationJob with the given context and payload.
func NewServiceOperationJob(ctx *JobContext, payload ServiceOperationPayload) *ServiceOperationJob {
	return &ServiceOperationJob{
		BaseJob: pkgjobs.NewBaseJob(ctx, payload),
	}
}

// NewServiceOperationTask creates an asynq task for performing a service operation
func NewServiceOperationTask(serverID, serviceID, operation string, userID *string) (*asynq.Task, error) {
	return pkgjobs.NewTask(TypeServiceOperation, ServiceOperationPayload{
		ServerID:  serverID,
		ServiceID: serviceID,
		Operation: operation,
		UserID:    userID,
	})
}
