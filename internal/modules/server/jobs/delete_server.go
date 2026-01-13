package jobs

import (
	"context"
	"fmt"
)

// DeleteServerJob deletes a server from the provider and database.
// Similar to Laravel's Modules\Server\Jobs\DeleteServer
type DeleteServerJob struct {
	ServerJobBase
	Payload DeleteServerPayload
}

// Type returns the job type identifier
func (j *DeleteServerJob) Type() string {
	return TypeDeleteServer
}

// Handle processes the job
func (j *DeleteServerJob) Handle(ctx context.Context) error {
	// Find the server
	server, err := j.Repo().FindServerByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	// TODO: Delete server from cloud provider if applicable
	// This would involve calling the provider's API to destroy the instance

	// Delete the server record from database
	if err := j.Repo().DeleteServer(ctx, server.ID); err != nil {
		return fmt.Errorf("failed to delete server: %w", err)
	}

	j.LogInfo("Server deleted successfully",
		"server_id", server.ID,
		"server_name", server.Name,
	)

	// Broadcast event
	j.BroadcastServerEvent(server.ID, "server.deleted", map[string]interface{}{
		"server_id": server.ID,
	})

	return nil
}

// Failed is called when the job fails after all retries
func (j *DeleteServerJob) Failed(ctx context.Context, err error) {
	j.LogError(err, "Failed to delete server",
		"server_id", j.Payload.ServerID,
	)
}
