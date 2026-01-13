package jobs

import (
	"context"
	"fmt"

	"github.com/kkz6/launch-go/internal/pkg/activity"
)

// ArchiveServerJob archives a server (soft delete).
// Similar to Laravel's Modules\Server\Jobs\ArchiveServer
type ArchiveServerJob struct {
	ServerJobBase
	Payload ArchiveServerPayload
}

// Type returns the job type identifier
func (j *ArchiveServerJob) Type() string {
	return TypeArchiveServer
}

// Handle processes the job
func (j *ArchiveServerJob) Handle(ctx context.Context) error {
	// Find the server
	server, err := j.Repo().FindServerByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	// Archive the server
	if err := j.Repo().ArchiveServer(ctx, server.ID); err != nil {
		return fmt.Errorf("failed to archive server: %w", err)
	}

	// Log activity
	logger := activity.New(j.DB).
		WithContext(ctx).
		UseLog("server").
		On(server).
		WithEvent("archived")
	if j.Payload.UserID != nil {
		logger.CausedByUser(*j.Payload.UserID)
	}
	logger.Log("Server was archived")

	j.LogInfo("Server archived successfully",
		"server_id", server.ID,
		"server_name", server.Name,
	)

	// Broadcast event
	j.BroadcastServerEvent(server.ID, "server.archived", map[string]any{
		"server_id": server.ID,
	})

	return nil
}

// Failed is called when the job fails after all retries
func (j *ArchiveServerJob) Failed(ctx context.Context, err error) {
	j.LogError(err, "Failed to archive server",
		"server_id", j.Payload.ServerID,
	)
}
