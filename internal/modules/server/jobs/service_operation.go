package jobs

import (
	"context"
	"fmt"

	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// ServiceOperationJob performs an operation (start/stop/restart/reload) on a service.
// Similar to Laravel's Modules\Server\Jobs\ServiceOperation
type ServiceOperationJob struct {
	ServerJobBase
	Payload ServiceOperationPayload
}

// Type returns the job type identifier
func (j *ServiceOperationJob) Type() string {
	return TypeServiceOperation
}

// Handle processes the job
func (j *ServiceOperationJob) Handle(ctx context.Context) error {
	// Find the service
	service, err := j.Repo().FindServiceByID(ctx, j.Payload.ServiceID)
	if err != nil {
		return fmt.Errorf("failed to find service: %w", err)
	}

	// Find the server
	server, err := j.Repo().FindServerByID(ctx, j.Payload.ServerID)
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

	result, err := j.RunTaskOnServer(server, task).
		AsRoot().
		Dispatch(ctx)

	if err != nil {
		return fmt.Errorf("failed to %s service: %w", j.Payload.Operation, err)
	}

	if !result.IsSuccessful() {
		return fmt.Errorf("failed to %s service: %s", j.Payload.Operation, result.GetOutput())
	}

	j.LogInfo("Service operation completed",
		"service_id", service.ID,
		"server_id", server.ID,
		"operation", j.Payload.Operation,
	)

	// Broadcast event
	j.BroadcastServerEvent(server.ID, "service.operation", map[string]any{
		"service_id": service.ID,
		"server_id":  server.ID,
		"operation":  j.Payload.Operation,
	})

	return nil
}

// Failed is called when the job fails after all retries
func (j *ServiceOperationJob) Failed(ctx context.Context, err error) {
	j.LogError(err, "Failed to perform service operation",
		"service_id", j.Payload.ServiceID,
		"server_id", j.Payload.ServerID,
		"operation", j.Payload.Operation,
	)
}
