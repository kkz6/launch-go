package jobs

import (
	"context"
	"fmt"

	"github.com/kkz6/launch-go/internal/modules/server/enums"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/pkg/jobs"
)

// RemoveServiceJob removes a service from a server.
// Similar to Laravel's Modules\Server\Jobs\UninstallService
type RemoveServiceJob struct {
	ServerJobBase
	jobs.UninstallationTracker
	Payload RemoveServicePayload
}

// Type returns the job type identifier
func (j *RemoveServiceJob) Type() string {
	return TypeRemoveService
}

// Handle processes the job
func (j *RemoveServiceJob) Handle(ctx context.Context) error {
	// Find the service with server
	service, err := j.Repo().FindServiceByID(ctx, j.Payload.ServiceID)
	if err != nil {
		return fmt.Errorf("failed to find service: %w", err)
	}

	// Find the server
	server, err := j.Repo().FindServerByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	// Create remove task using the software's remove template
	task := tasks.RemoveSoftware(service.GetSoftware())

	result, err := j.RunTaskOnServer(server, task).
		AsRoot().
		Dispatch(ctx)

	if err != nil {
		return fmt.Errorf("failed to remove service: %w", err)
	}

	if !result.IsSuccessful() {
		j.LogError(nil, "Service removal completed with errors",
			"output", result.GetOutput())
	}

	// Delete the service record
	if err := j.Repo().DeleteService(ctx, service.ID); err != nil {
		return fmt.Errorf("failed to delete service record: %w", err)
	}

	j.LogInfo("Service removed successfully",
		"service_id", service.ID,
		"server_id", server.ID,
	)

	// Broadcast event
	j.BroadcastServerEvent(server.ID, "service.removed", map[string]interface{}{
		"service_id": service.ID,
		"server_id":  server.ID,
	})

	return nil
}

// Failed is called when the job fails after all retries
func (j *RemoveServiceJob) Failed(ctx context.Context, err error) {
	j.LogError(err, "Failed to remove service",
		"service_id", j.Payload.ServiceID,
		"server_id", j.Payload.ServerID,
	)

	// Update service status to failed
	_ = j.Repo().UpdateServiceStatus(ctx, j.Payload.ServiceID, enums.ServiceStatusFailed)
}
