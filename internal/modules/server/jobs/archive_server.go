package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/pkg/activity"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeArchiveServer = "server:archive"

type ArchiveServerPayload struct {
	ServerID string  `json:"server_id"`
	UserID   *string `json:"user_id,omitempty"`
}

// ArchiveServerJob archives a server (soft delete).
// Similar to Laravel's Modules\Server\Jobs\ArchiveServer
type ArchiveServerJob struct {
	ctx     *JobContext
	Payload ArchiveServerPayload
}

// Handle processes the job
func (j *ArchiveServerJob) Handle(ctx context.Context) error {
	// Find the server
	server, err := j.ctx.Repos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	// Archive the server
	if err := j.ctx.Repos.Server().Archive(ctx, server.ID); err != nil {
		return fmt.Errorf("failed to archive server: %w", err)
	}

	// Log activity
	logger := activity.New(j.ctx.DB).
		WithContext(ctx).
		UseLog("server").
		On(server).
		WithEvent("archived")
	if j.Payload.UserID != nil {
		logger.CausedByUser(*j.Payload.UserID)
	}
	logger.Log("Server was archived")

	j.ctx.LogInfo("Server archived successfully",
		"server_id", server.ID,
		"server_name", server.Name,
	)

	// Broadcast event
	j.ctx.BroadcastServerEvent(server, "server.archived", map[string]any{
		"server_id": server.ID,
	})

	return nil
}

// Failed is called when the job fails after all retries
func (j *ArchiveServerJob) Failed(ctx context.Context, err error) {
	j.ctx.LogError(err, "Failed to archive server",
		"server_id", j.Payload.ServerID,
	)
}

// NewArchiveServerJob creates a new ArchiveServerJob with the given context and payload.
func NewArchiveServerJob(ctx *JobContext, payload ArchiveServerPayload) *ArchiveServerJob {
	return &ArchiveServerJob{
		ctx:     ctx,
		Payload: payload,
	}
}

// NewArchiveServerTask creates an asynq task for archiving a server
// Uses TaskID for deduplication to prevent duplicate archive operations
func NewArchiveServerTask(serverID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.NewTask(TypeArchiveServer, ArchiveServerPayload{
		ServerID: serverID,
		UserID:   userID,
	}, asynq.TaskID(fmt.Sprintf("archive_server:%s", serverID)))
}
