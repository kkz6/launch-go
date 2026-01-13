package jobs

import (
	"context"
	"fmt"
)

// UnarchiveServerJob unarchives a server (restore from soft delete).
// Similar to Laravel's Modules\Server\Jobs\UnarchiveServer
type UnarchiveServerJob struct {
	ServerJobBase
	Payload UnarchiveServerPayload
}

// Type returns the job type identifier
func (j *UnarchiveServerJob) Type() string {
	return TypeUnarchiveServer
}

// Handle processes the job
func (j *UnarchiveServerJob) Handle(ctx context.Context) error {
	// Find the server (including archived)
	server, err := j.Repo().FindServerByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	// Unarchive the server
	if err := j.Repo().UnarchiveServer(ctx, server.ID); err != nil {
		return fmt.Errorf("failed to unarchive server: %w", err)
	}

	j.LogInfo("Server unarchived successfully",
		"server_id", server.ID,
		"server_name", server.Name,
	)

	// Broadcast event
	j.BroadcastServerEvent(server.ID, "server.unarchived", map[string]any{
		"server_id": server.ID,
	})

	return nil
}

// Failed is called when the job fails after all retries
func (j *UnarchiveServerJob) Failed(ctx context.Context, err error) {
	j.LogError(err, "Failed to unarchive server",
		"server_id", j.Payload.ServerID,
	)
}
