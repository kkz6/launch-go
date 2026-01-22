package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	"github.com/kkz6/launch-go/internal/pkg/launch/activity"
)

const TypeUnarchiveServer = "server:unarchive"

type UnarchiveServerPayload struct {
	ServerID string  `json:"server_id"`
	UserID   *string `json:"user_id,omitempty"`
}

// UnarchiveServerJob unarchives a server (restore from soft delete).
// Similar to Laravel's Modules\Server\Jobs\UnarchiveServer
type UnarchiveServerJob struct {
	pkgjobs.BaseJob[*JobContext, UnarchiveServerPayload]
}

// Handle processes the job
func (j *UnarchiveServerJob) Handle(ctx context.Context) error {
	// Find the server (including archived)
	server, err := j.Ctx.Repos().Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	// Unarchive the server
	if err := j.Ctx.Repos().Server().Unarchive(ctx, server.ID); err != nil {
		return fmt.Errorf("failed to unarchive server: %w", err)
	}

	// Log activity
	activity.RecordWithLogPtr(ctx, "server", "unarchived", j.Payload.UserID, server, "Server was unarchived")

	j.Ctx.LogInfo("Server unarchived successfully",
		"server_id", server.ID,
		"server_name", server.Name,
	)

	// Broadcast event
	j.Ctx.BroadcastServerEvent(server, "server.unarchived", map[string]any{
		"server_id": server.ID,
	})

	return nil
}

// Failed is called when the job fails after all retries
func (j *UnarchiveServerJob) Failed(ctx context.Context, err error) {
	j.Ctx.LogError(err, "Failed to unarchive server",
		"server_id", j.Payload.ServerID,
	)
}

// NewUnarchiveServerJob creates a new UnarchiveServerJob with the given context and payload.
func NewUnarchiveServerJob(ctx *JobContext, payload UnarchiveServerPayload) *UnarchiveServerJob {
	return &UnarchiveServerJob{
		BaseJob: pkgjobs.NewBaseJob(ctx, payload),
	}
}

// NewUnarchiveServerTask creates an asynq task for unarchiving a server
// Uses TaskID for deduplication to prevent duplicate unarchive operations
func NewUnarchiveServerTask(serverID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.NewTask(TypeUnarchiveServer, UnarchiveServerPayload{
		ServerID: serverID,
		UserID:   userID,
	}, asynq.TaskID(fmt.Sprintf("unarchive_server:%s", serverID)))
}
