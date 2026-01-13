package jobs

import (
	"context"
	"fmt"

	"github.com/kkz6/launch-go/internal/modules/server/tasks"
)

// UpdateConnectivityJob checks and updates the connectivity status of a server.
// Similar to Laravel's Modules\Server\Jobs\UpdateConnectivity
type UpdateConnectivityJob struct {
	ServerJobBase
	Payload UpdateConnectivityPayload
}

// Type returns the job type identifier
func (j *UpdateConnectivityJob) Type() string {
	return TypeUpdateConnectivity
}

// Handle processes the job
func (j *UpdateConnectivityJob) Handle(ctx context.Context) error {
	// Find the server
	server, err := j.Repo().FindServerByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	// Run a simple connectivity check (whoami)
	task := tasks.Whoami()

	result, err := j.RunTaskOnServer(server, task).
		AsRoot().
		Dispatch(ctx)

	isConnected := err == nil && result != nil && result.IsSuccessful()

	// Update server connectivity status
	if err := j.Repo().UpdateServerFields(ctx, server.ID, map[string]interface{}{
		"is_connected": isConnected,
	}); err != nil {
		return fmt.Errorf("failed to update connectivity status: %w", err)
	}

	j.LogInfo("Server connectivity updated",
		"server_id", server.ID,
		"is_connected", isConnected,
	)

	// Broadcast event
	j.BroadcastServerEvent(server.ID, "server.connectivity", map[string]interface{}{
		"server_id":    server.ID,
		"is_connected": isConnected,
	})

	return nil
}

// Failed is called when the job fails after all retries
func (j *UpdateConnectivityJob) Failed(ctx context.Context, err error) {
	j.LogError(err, "Failed to update connectivity",
		"server_id", j.Payload.ServerID,
	)

	// Mark server as disconnected on failure
	_ = j.Repo().UpdateServerFields(ctx, j.Payload.ServerID, map[string]interface{}{
		"is_connected": false,
	})
}
