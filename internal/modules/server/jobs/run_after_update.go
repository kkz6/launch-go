package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeRunAfterUpdate = "server:run_after_update"

// RunAfterUpdatePayload holds data for post-update commands
type RunAfterUpdatePayload struct {
	ServerID string  `json:"server_id"`
	UserID   *string `json:"user_id,omitempty"`
}

// RunAfterUpdateJob runs commands after a server update event
type RunAfterUpdateJob struct {
	pkgjobs.BaseJob[*JobContext, RunAfterUpdatePayload]
}

// NewRunAfterUpdateJob creates a new RunAfterUpdateJob
func NewRunAfterUpdateJob(ctx *JobContext, payload RunAfterUpdatePayload) *RunAfterUpdateJob {
	return &RunAfterUpdateJob{
		BaseJob: pkgjobs.NewBaseJob(ctx, payload),
	}
}

// Handle executes the post-update job
func (j *RunAfterUpdateJob) Handle(ctx context.Context) error {
	server, err := j.Ctx.Repos().Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	j.Ctx.LogInfo("Running post-update commands",
		"server_id", server.ID,
	)

	// Run post-update tasks
	// 1. Restart any services that may need it after update
	// 2. Clear any caches
	// 3. Reload configurations

	// Reload supervisor to pick up any config changes
	reloadTask := tasks.ReloadSupervisor()
	if result, err := j.Ctx.ForServer(server).RunTask(reloadTask).AsRoot().Dispatch(ctx); err != nil {
		j.Ctx.LogError(err, "Failed to reload supervisor")
	} else if !result.IsSuccessful() {
		j.Ctx.LogError(nil, "Supervisor reload returned non-zero exit code", "exit_code", result.GetExitCode())
	}

	// Reload Caddy configuration
	caddyTask := tasks.ReloadCaddy()
	if result, err := j.Ctx.ForServer(server).RunTask(caddyTask).AsRoot().Dispatch(ctx); err != nil {
		j.Ctx.LogError(err, "Failed to reload Caddy")
	} else if !result.IsSuccessful() {
		j.Ctx.LogError(nil, "Caddy reload returned non-zero exit code", "exit_code", result.GetExitCode())
	}

	// Clear OPcache if PHP is installed
	clearOpcacheTask := tasks.ClearOpcache()
	if _, err := j.Ctx.ForServer(server).RunTask(clearOpcacheTask).AsRoot().Dispatch(ctx); err != nil {
		// OPcache clear failure is not critical, just log it
		j.Ctx.LogInfo("OPcache clear skipped or failed (may not be installed)")
	}

	j.Ctx.LogInfo("Post-update commands completed",
		"server_id", server.ID,
	)

	j.Ctx.BroadcastServerEvent(server, "server.post_update_completed", map[string]any{
		"server_id": server.ID,
	})

	return nil
}

// Failed handles job failure
func (j *RunAfterUpdateJob) Failed(ctx context.Context, err error) {
	j.Ctx.LogError(err, "Post-update commands failed",
		"server_id", j.Payload.ServerID,
	)
}

// NewRunAfterUpdateTask creates a post-update task
func NewRunAfterUpdateTask(serverID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.NewTask(TypeRunAfterUpdate, RunAfterUpdatePayload{
		ServerID: serverID,
		UserID:   userID,
	})
}
