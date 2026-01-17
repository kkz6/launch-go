package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/pkg/activity"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeRebootServer = "server:reboot"

type RebootServerPayload struct {
	ServerID string  `json:"server_id"`
	UserID   *string `json:"user_id,omitempty"`
}

// RebootServerJob reboots a server.
// Similar to Laravel's Modules\Server\Jobs\RebootServer
type RebootServerJob struct {
	ctx     *JobContext
	Payload RebootServerPayload
}

// Handle processes the job
func (j *RebootServerJob) Handle(ctx context.Context) error {
	// Find the server
	server, err := j.ctx.Repos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	// Create reboot task
	task := tasks.RebootServer()

	// Execute reboot - don't wait for result as server will disconnect
	_, err = j.ctx.ForServer(server).RunTask(task).
		AsRoot().
		Dispatch(ctx)

	// Reboot command may cause connection to drop, which is expected
	if err != nil {
		j.ctx.LogInfo("Reboot command sent, connection dropped as expected",
			"server_id", server.ID,
		)
	}

	// Log activity
	logger := activity.New(j.ctx.DB).
		WithContext(ctx).
		UseLog("server").
		On(server).
		WithEvent("rebooted")
	if j.Payload.UserID != nil {
		logger.CausedByUser(*j.Payload.UserID)
	}
	logger.Log("Server reboot was initiated")

	j.ctx.LogInfo("Server reboot initiated",
		"server_id", server.ID,
		"server_name", server.Name,
	)

	// Broadcast event
	j.ctx.BroadcastServerEvent(server, "server.rebooting", map[string]any{
		"server_id": server.ID,
	})

	return nil
}

// Failed is called when the job fails after all retries
func (j *RebootServerJob) Failed(ctx context.Context, err error) {
	j.ctx.LogError(err, "Failed to reboot server",
		"server_id", j.Payload.ServerID,
	)
}

func NewRebootServerJob(ctx *JobContext, payload RebootServerPayload) *RebootServerJob {
	return &RebootServerJob{
		ctx:     ctx,
		Payload: payload,
	}
}

// NewRebootServerTask creates an asynq task for rebooting a server
// Uses TaskID for deduplication to prevent duplicate reboots
func NewRebootServerTask(serverID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.NewTask(TypeRebootServer, RebootServerPayload{
		ServerID: serverID,
		UserID:   userID,
	}, asynq.TaskID(fmt.Sprintf("reboot:%s", serverID)))
}
