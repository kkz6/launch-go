package jobs

import (
	"context"
	"fmt"

	"github.com/kkz6/launch-go/internal/modules/server/enums"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/pkg/jobs"
)

// AddServiceJob installs a service (software) on a server.
// Similar to Laravel's Modules\Server\Jobs\InstallService
type AddServiceJob struct {
	ServerJobBase
	jobs.InstallationTracker
	Payload AddServicePayload
}

// Type returns the job type identifier
func (j *AddServiceJob) Type() string {
	return TypeAddService
}

// Handle processes the job
func (j *AddServiceJob) Handle(ctx context.Context) error {
	// Find the service with server preloaded
	service, err := j.Repo().FindServiceByID(ctx, j.Payload.ServiceID)
	if err != nil {
		return fmt.Errorf("failed to find service: %w", err)
	}

	// Find the server
	server, err := j.Repo().FindServerByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	// Update service status to installing
	if err := j.Repo().UpdateServiceStatus(ctx, service.ID, enums.ServiceStatusInstalling); err != nil {
		return fmt.Errorf("failed to update service status: %w", err)
	}

	// Get software type from payload
	software := enums.Software(j.Payload.Software)

	// Create install task using the software's install template
	task := tasks.InstallSoftware(software)

	result, err := j.RunTaskOnServer(server, task).
		AsRoot().
		TrackInDB().
		Dispatch(ctx)

	if err != nil {
		return fmt.Errorf("failed to install service: %w", err)
	}

	if !result.IsSuccessful() {
		return fmt.Errorf("failed to install service: %s", result.GetOutput())
	}

	// Update service status to running
	if err := j.Repo().UpdateServiceStatus(ctx, service.ID, enums.ServiceStatusRunning); err != nil {
		return fmt.Errorf("failed to update service status: %w", err)
	}

	j.LogInfo("Service installed successfully",
		"service_id", service.ID,
		"server_id", server.ID,
		"software", j.Payload.Software,
	)

	// Broadcast event
	j.BroadcastServerEvent(server.ID, "service.installed", map[string]interface{}{
		"service_id": service.ID,
		"server_id":  server.ID,
		"software":   j.Payload.Software,
	})

	return nil
}

// Failed is called when the job fails after all retries
func (j *AddServiceJob) Failed(ctx context.Context, err error) {
	j.LogError(err, "Failed to install service",
		"service_id", j.Payload.ServiceID,
		"server_id", j.Payload.ServerID,
	)

	// Update service status to failed
	_ = j.Repo().UpdateServiceStatus(ctx, j.Payload.ServiceID, enums.ServiceStatusFailed)
}
