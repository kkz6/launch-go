package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeCheckDaemonStatus = "server:check_daemon_status"

// CheckDaemonStatusPayload holds data for daemon status check
type CheckDaemonStatusPayload struct {
	ServerID string  `json:"server_id"`
	UserID   *string `json:"user_id,omitempty"`
}

// CheckDaemonStatusJob checks the status of all daemons on a server
// This is typically triggered by a scheduled command
type CheckDaemonStatusJob struct {
	pkgjobs.BaseJob[*JobContext, CheckDaemonStatusPayload]
}

// NewCheckDaemonStatusJob creates a new CheckDaemonStatusJob
func NewCheckDaemonStatusJob(ctx *JobContext, payload CheckDaemonStatusPayload) *CheckDaemonStatusJob {
	return &CheckDaemonStatusJob{
		BaseJob: pkgjobs.NewBaseJob(ctx, payload),
	}
}

// Handle executes the daemon status check job
// This job delegates to SyncDaemons for the actual work
func (j *CheckDaemonStatusJob) Handle(ctx context.Context) error {
	server, err := j.Ctx.Repos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	j.Ctx.LogInfo("Checking daemon status",
		"server_id", server.ID,
	)

	// Create and execute the sync daemons job directly
	syncJob := NewSyncDaemonsJob(j.Ctx, SyncDaemonsPayload{
		ServerID: j.Payload.ServerID,
		UserID:   j.Payload.UserID,
	})

	if err := syncJob.Handle(ctx); err != nil {
		return fmt.Errorf("failed to sync daemon status: %w", err)
	}

	j.Ctx.LogInfo("Daemon status check completed",
		"server_id", server.ID,
	)

	return nil
}

// Failed handles job failure
func (j *CheckDaemonStatusJob) Failed(ctx context.Context, err error) {
	j.Ctx.LogError(err, "Daemon status check failed",
		"server_id", j.Payload.ServerID,
	)
}

// NewCheckDaemonStatusTask creates a daemon status check task
func NewCheckDaemonStatusTask(serverID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.NewTask(TypeCheckDaemonStatus, CheckDaemonStatusPayload{
		ServerID: serverID,
		UserID:   userID,
	})
}
