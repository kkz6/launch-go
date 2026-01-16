package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeUpdateConnectivity = "server:update_connectivity"

type UpdateConnectivityPayload struct {
	ServerID string `json:"server_id"`
}

// UpdateConnectivityJob checks and updates the connectivity status of a server.
// Similar to Laravel's Modules\Server\Jobs\UpdateConnectivity
type UpdateConnectivityJob struct {
	ctx     *JobContext
	Payload UpdateConnectivityPayload
}

// Handle processes the job
func (j *UpdateConnectivityJob) Handle(ctx context.Context) error {
	// Find the server
	server, err := j.ctx.Repo.FindServerByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	// Run a simple connectivity check (whoami)
	task := tasks.Whoami()

	result, err := j.ctx.ForServer(server).RunTask(task).
		AsRoot().
		Dispatch(ctx)

	isConnected := err == nil && result != nil && result.IsSuccessful()

	// Update server connectivity status
	if err := j.ctx.Repo.UpdateServerFields(ctx, server.ID, map[string]any{
		"is_connected": isConnected,
	}); err != nil {
		return fmt.Errorf("failed to update connectivity status: %w", err)
	}

	j.ctx.LogInfo("Server connectivity updated",
		"server_id", server.ID,
		"is_connected", isConnected,
	)

	// Broadcast event
	j.ctx.BroadcastToServer(server.ID, "server.connectivity", map[string]any{
		"server_id":    server.ID,
		"is_connected": isConnected,
	})

	return nil
}

// Failed is called when the job fails after all retries
func (j *UpdateConnectivityJob) Failed(ctx context.Context, err error) {
	j.ctx.LogError(err, "Failed to update connectivity",
		"server_id", j.Payload.ServerID,
	)

	// Mark server as disconnected on failure
	_ = j.ctx.Repo.UpdateServerFields(ctx, j.Payload.ServerID, map[string]any{
		"is_connected": false,
	})
}

// NewUpdateConnectivityJob creates a new UpdateConnectivityJob with the given context and payload.
func NewUpdateConnectivityJob(ctx *JobContext, payload UpdateConnectivityPayload) *UpdateConnectivityJob {
	return &UpdateConnectivityJob{
		ctx:     ctx,
		Payload: payload,
	}
}

// NewUpdateConnectivityTask creates an asynq task for updating server connectivity
func NewUpdateConnectivityTask(serverID string) (*asynq.Task, error) {
	return pkgjobs.NewTask(TypeUpdateConnectivity, UpdateConnectivityPayload{
		ServerID: serverID,
	})
}
