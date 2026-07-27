package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/modules/server/types"
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
	case "update":
		// In-place upgrade. Only the Launch Agent supports it: re-run its
		// installer (binary swap) + restart, without touching config or
		// the systemd unit. Other software has no update path here.
		if j.service.GetSoftware() != types.SoftwareLaunchAgent {
			return fmt.Errorf("update operation is only supported for the launch agent")
		}
		task = tasks.UpdateLaunchAgent()
	default:
		return fmt.Errorf("unknown operation: %s", j.Payload.Operation)
	}

	var taskID string
	result, err := j.Deps.RunTask(j.server, task).
		AsRoot().
		TrackInDB().
		OnTaskCreated(func(id string) {
			taskID = id
			j.Deps.BroadcastServerEvent(j.server, "service.operation", map[string]any{
				"service_id": j.service.ID,
				"operation":  j.Payload.Operation,
				"task_id":    id,
				"status":     "running",
			})
		}).
		Dispatch(ctx)

	if err != nil {
		j.broadcastFailure(taskID, err.Error())
		return fmt.Errorf("failed to %s service: %w", j.Payload.Operation, err)
	}

	if !result.IsSuccessful() {
		j.broadcastFailure(taskID, result.GetOutput())
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
		"task_id":    taskID,
		"status":     "finished",
	})

	return nil
}

// broadcastFailure makes the terminal state and the persisted script log
// discoverable to the server UI even when the queue retries the job.
func (j *ServiceOperationJob) broadcastFailure(taskID, output string) {
	if j.server == nil || j.service == nil {
		return
	}
	j.Deps.BroadcastServerEvent(j.server, "service.operation", map[string]any{
		"service_id": j.service.ID,
		"operation":  j.Payload.Operation,
		"task_id":    taskID,
		"status":     "failed",
		"output":     output,
	})
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
