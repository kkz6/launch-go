package jobs

import (
	"context"
	"fmt"

	"github.com/kkz6/launch-go/internal/modules/server/enums"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
)

// ConfigureOpcacheJob configures OPcache settings for a PHP version.
type ConfigureOpcacheJob struct {
	ServerJobBase
	Payload ConfigureOpcachePayload
}

// Type returns the job type identifier
func (j *ConfigureOpcacheJob) Type() string {
	return TypeConfigureOpcache
}

// Handle processes the job
func (j *ConfigureOpcacheJob) Handle(ctx context.Context) error {
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

	// Verify it's a PHP service
	if service.Type != enums.ServiceTypePhp {
		return fmt.Errorf("service is not a PHP installation")
	}

	// Get the PHP version from the software
	software := enums.Software(service.Software)
	version := software.GetVersion()

	// Create and run the configure opcache task
	task := tasks.ConfigureOpcache(version, j.Payload.Settings)
	result, err := j.RunTaskOnServer(server, task).
		AsRoot().
		TrackInDB().
		Dispatch(ctx)

	if err != nil {
		return fmt.Errorf("failed to configure OPcache: %w", err)
	}

	if !result.IsSuccessful() {
		return fmt.Errorf("failed to configure OPcache: %s", result.GetOutput())
	}

	j.LogInfo("OPcache configuration completed",
		"service_id", service.ID,
		"server_id", server.ID,
	)

	// Broadcast event
	j.BroadcastServerEvent(server.ID, "opcache.configured", map[string]any{
		"service_id": service.ID,
		"server_id":  server.ID,
	})

	return nil
}

// Failed is called when the job fails after all retries
func (j *ConfigureOpcacheJob) Failed(ctx context.Context, err error) {
	j.LogError(err, "Failed to configure OPcache",
		"service_id", j.Payload.ServiceID,
		"server_id", j.Payload.ServerID,
	)
}
