package jobs

import (
	"context"
	"fmt"

	"github.com/kkz6/launch-go/internal/modules/server/tasks"
)

// RebootServerJob reboots a server.
// Similar to Laravel's Modules\Server\Jobs\RebootServer
type RebootServerJob struct {
	ServerJobBase
	Payload RebootServerPayload
}

// Type returns the job type identifier
func (j *RebootServerJob) Type() string {
	return TypeRebootServer
}

// Handle processes the job
func (j *RebootServerJob) Handle(ctx context.Context) error {
	// Find the server
	server, err := j.Repo().FindServerByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	// Create reboot task
	task := tasks.RebootServer()

	// Execute reboot - don't wait for result as server will disconnect
	_, err = j.RunTaskOnServer(server, task).
		AsRoot().
		Dispatch(ctx)

	// Reboot command may cause connection to drop, which is expected
	if err != nil {
		j.LogInfo("Reboot command sent, connection dropped as expected",
			"server_id", server.ID,
		)
	}

	j.LogInfo("Server reboot initiated",
		"server_id", server.ID,
		"server_name", server.Name,
	)

	// Broadcast event
	j.BroadcastServerEvent(server.ID, "server.rebooting", map[string]any{
		"server_id": server.ID,
	})

	return nil
}

// Failed is called when the job fails after all retries
func (j *RebootServerJob) Failed(ctx context.Context, err error) {
	j.LogError(err, "Failed to reboot server",
		"server_id", j.Payload.ServerID,
	)
}
