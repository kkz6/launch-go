package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/pkg/activity"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeUnarchiveServer = "server:unarchive"

type UnarchiveServerPayload struct {
	ServerID string  `json:"server_id"`
	UserID   *string `json:"user_id,omitempty"`
}

// UnarchiveServerJob unarchives a server (restore from soft delete).
// Similar to Laravel's Modules\Server\Jobs\UnarchiveServer
type UnarchiveServerJob struct {
	ctx     *JobContext
	Payload UnarchiveServerPayload
}

// Handle processes the job
func (j *UnarchiveServerJob) Handle(ctx context.Context) error {
	// Find the server (including archived)
	server, err := j.ctx.Repo.FindServerByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	// Unarchive the server
	if err := j.ctx.Repo.UnarchiveServer(ctx, server.ID); err != nil {
		return fmt.Errorf("failed to unarchive server: %w", err)
	}

	// Log activity
	logger := activity.New(j.ctx.DB).
		WithContext(ctx).
		UseLog("server").
		On(server).
		WithEvent("unarchived")
	if j.Payload.UserID != nil {
		logger.CausedByUser(*j.Payload.UserID)
	}
	logger.Log("Server was unarchived")

	j.ctx.LogInfo("Server unarchived successfully",
		"server_id", server.ID,
		"server_name", server.Name,
	)

	// Broadcast event
	j.ctx.BroadcastToServer(server.ID, "server.unarchived", map[string]any{
		"server_id": server.ID,
	})

	return nil
}

// Failed is called when the job fails after all retries
func (j *UnarchiveServerJob) Failed(ctx context.Context, err error) {
	j.ctx.LogError(err, "Failed to unarchive server",
		"server_id", j.Payload.ServerID,
	)
}

// NewUnarchiveServerJob creates a new UnarchiveServerJob with the given context and payload.
func NewUnarchiveServerJob(ctx *JobContext, payload UnarchiveServerPayload) *UnarchiveServerJob {
	return &UnarchiveServerJob{
		ctx:     ctx,
		Payload: payload,
	}
}

// NewUnarchiveServerTask creates an asynq task for unarchiving a server
func NewUnarchiveServerTask(serverID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.NewTask(TypeUnarchiveServer, UnarchiveServerPayload{
		ServerID: serverID,
		UserID:   userID,
	})
}
