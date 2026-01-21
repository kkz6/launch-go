package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/pkg/activity"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeRemoveService = "server:remove_service"

type RemoveServicePayload struct {
	ServerID  string  `json:"server_id"`
	ServiceID string  `json:"service_id"`
	UserID    *string `json:"user_id,omitempty"`
}

// RemoveServiceJob removes a service from a server.
// Similar to Laravel's Modules\Server\Jobs\UninstallService
type RemoveServiceJob struct {
	pkgjobs.BaseJob[*JobContext, RemoveServicePayload]
}

// Handle processes the job
func (j *RemoveServiceJob) Handle(ctx context.Context) error {
	// Find the service with server
	service, err := j.Ctx.Repos.Service().FindByID(ctx, j.Payload.ServiceID)
	if err != nil {
		return fmt.Errorf("failed to find service: %w", err)
	}

	// Find the server
	server, err := j.Ctx.Repos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	// Create remove task using the software's remove template
	task := tasks.RemoveSoftware(service.GetSoftware())

	result, err := j.Ctx.ForServer(server).RunTask(task).
		AsRoot().
		Dispatch(ctx)

	if err != nil {
		return fmt.Errorf("failed to remove service: %w", err)
	}

	if !result.IsSuccessful() {
		j.Ctx.LogError(nil, "Service removal completed with errors",
			"output", result.GetOutput())
	}

	// Log activity before deletion
	activity.LogWithLogPtr(ctx, j.Ctx.DB, "server", "removed", j.Payload.UserID, service, "Service was removed")

	// Delete the service record
	if err := j.Ctx.Repos.Service().Delete(ctx, service.ID); err != nil {
		return fmt.Errorf("failed to delete service record: %w", err)
	}

	j.Ctx.LogInfo("Service removed successfully",
		"service_id", service.ID,
		"server_id", server.ID,
	)

	// Broadcast event
	j.Ctx.BroadcastServerEvent(server, "service.removed", map[string]any{
		"service_id": service.ID,
		"server_id":  server.ID,
	})

	return nil
}

// Failed is called when the job fails after all retries
func (j *RemoveServiceJob) Failed(ctx context.Context, err error) {
	j.Ctx.LogError(err, "Failed to remove service",
		"service_id", j.Payload.ServiceID,
		"server_id", j.Payload.ServerID,
	)

	// Mark removal as failed
	service, findErr := j.Ctx.Repos.Service().FindByID(ctx, j.Payload.ServiceID)
	if findErr == nil && service != nil {
		now := time.Now()
		j.Ctx.DB.Model(service).Updates(map[string]any{
			"removal_requested_at": nil,
			"removal_failed_at":    &now,
		})
	}
}

func NewRemoveServiceJob(ctx *JobContext, payload RemoveServicePayload) *RemoveServiceJob {
	return &RemoveServiceJob{
		BaseJob: pkgjobs.NewBaseJob(ctx, payload),
	}
}

// NewRemoveServiceTask creates an asynq task for removing a service
// Uses TaskID for deduplication to prevent duplicate service removals
func NewRemoveServiceTask(serverID, serviceID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.NewTask(TypeRemoveService, RemoveServicePayload{
		ServerID:  serverID,
		ServiceID: serviceID,
		UserID:    userID,
	}, asynq.TaskID(fmt.Sprintf("remove_service:%s:%s", serverID, serviceID)))
}
