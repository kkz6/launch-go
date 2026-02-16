package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/models"
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
	Deps    *JobDeps
	Payload RunAfterUpdatePayload

	server *models.Server
}

func NewRunAfterUpdateJob(p RunAfterUpdatePayload) pkgjobs.Handler {
	return &RunAfterUpdateJob{Deps: deps, Payload: p}
}

// Handle executes the post-update job
func (j *RunAfterUpdateJob) Handle(ctx context.Context) error {
	var err error
	j.server, err = j.Deps.Repos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	j.Deps.Logger.Info().
		Str("server_id", j.server.ID).
		Msg("running post-update commands")

	// Run post-update tasks
	// 1. Restart any services that may need it after update
	// 2. Clear any caches
	// 3. Reload configurations

	// Reload supervisor to pick up any config changes
	reloadTask := tasks.ReloadSupervisor()
	if result, err := j.Deps.RunTask(j.server, reloadTask).AsUser().Dispatch(ctx); err != nil {
		j.Deps.Logger.Error().Err(err).
			Msg("failed to reload supervisor")
	} else if !result.IsSuccessful() {
		j.Deps.Logger.Error().
			Int("exit_code", result.GetExitCode()).
			Msg("supervisor reload returned non-zero exit code")
	}

	// Reload Caddy configuration
	caddyTask := tasks.ReloadCaddy()
	if result, err := j.Deps.RunTask(j.server, caddyTask).AsUser().Dispatch(ctx); err != nil {
		j.Deps.Logger.Error().Err(err).
			Msg("failed to reload Caddy")
	} else if !result.IsSuccessful() {
		j.Deps.Logger.Error().
			Int("exit_code", result.GetExitCode()).
			Msg("Caddy reload returned non-zero exit code")
	}

	// Clear OPcache if PHP is installed
	clearOpcacheTask := tasks.ClearOpcache()
	if _, err := j.Deps.RunTask(j.server, clearOpcacheTask).AsUser().Dispatch(ctx); err != nil {
		// OPcache clear failure is not critical, just log it
		j.Deps.Logger.Info().
			Msg("OPcache clear skipped or failed (may not be installed)")
	}

	j.Deps.Logger.Info().
		Str("server_id", j.server.ID).
		Msg("post-update commands completed")

	j.Deps.BroadcastServerEvent(j.server, "server.post_update_completed", map[string]any{
		"server_id": j.server.ID,
	})

	return nil
}

// Failed handles job failure
func (j *RunAfterUpdateJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).
		Str("server_id", j.Payload.ServerID).
		Msg("post-update commands failed")
}

// NewRunAfterUpdateTask creates a post-update task
func NewRunAfterUpdateTask(serverID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.Task(TypeRunAfterUpdate, RunAfterUpdatePayload{
		ServerID: serverID,
		UserID:   userID,
	})
}
