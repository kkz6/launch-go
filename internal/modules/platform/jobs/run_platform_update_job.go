package jobs

import (
	"context"
	"fmt"

	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"

	"github.com/kkz6/launch-go/internal/modules/platform/tasks"
	"github.com/kkz6/launch-go/internal/modules/platform/types"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeRunPlatformUpdate = "platform:run_update"

// RunPlatformUpdatePayload is the payload for the run platform update job
type RunPlatformUpdatePayload struct {
	ServerID               string `json:"server_id"`
	PlatformUpdateID       string `json:"platform_update_id"`
	ServerPlatformUpdateID string `json:"server_platform_update_id"`
	TeamID                 string `json:"team_id"`
}

// RunPlatformUpdateJob executes a platform update task on a server
type RunPlatformUpdateJob struct {
	Deps    *JobDeps
	Payload RunPlatformUpdatePayload
}

// NewRunPlatformUpdateJob creates a new RunPlatformUpdateJob
func NewRunPlatformUpdateJob(p RunPlatformUpdatePayload) pkgjobs.Handler {
	return &RunPlatformUpdateJob{Deps: deps, Payload: p}
}

// Handle executes the job
func (j *RunPlatformUpdateJob) Handle(ctx context.Context) error {
	// Load server
	var server servermodels.Server
	if err := j.Deps.DB.WithContext(ctx).First(&server, "id = ?", j.Payload.ServerID).Error; err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	// Load platform update
	update, err := j.Deps.Repos.PlatformUpdate().FindByID(ctx, j.Payload.PlatformUpdateID)
	if err != nil {
		return fmt.Errorf("failed to find platform update: %w", err)
	}

	// Look up task factory from registry
	factory, ok := tasks.GetUpdateTask(update.Key)
	if !ok {
		return fmt.Errorf("no task factory registered for update key: %s", update.Key)
	}

	// Set status to running
	if err := j.Deps.Repos.ServerPlatformUpdate().UpdateStatus(
		ctx, j.Payload.ServerPlatformUpdateID, types.UpdateStatusRunning,
	); err != nil {
		return fmt.Errorf("failed to update status to running: %w", err)
	}

	// Broadcast status change
	j.Deps.BroadcastToTeam(j.Payload.TeamID, "platform_update.status_changed", map[string]interface{}{
		"update_id": j.Payload.ServerPlatformUpdateID,
		"server_id": j.Payload.ServerID,
		"status":    "running",
	})

	// Create the task
	task := factory(&server, update)

	// Set the server platform update ID for callback tracking
	if setter, ok := task.(interface{ SetServerPlatformUpdateID(string) }); ok {
		setter.SetServerPlatformUpdateID(j.Payload.ServerPlatformUpdateID)
	}

	// Run the task as root (rename requires root privileges)
	_, err = j.Deps.RunTask(&server, task).
		AsRoot().
		TrackInDB().
		RunAsync(ctx)
	if err != nil {
		// Update status to failed
		j.Deps.Repos.ServerPlatformUpdate().UpdateStatus(
			ctx, j.Payload.ServerPlatformUpdateID, types.UpdateStatusFailed,
		)

		j.Deps.BroadcastToTeam(j.Payload.TeamID, "platform_update.status_changed", map[string]interface{}{
			"update_id": j.Payload.ServerPlatformUpdateID,
			"server_id": j.Payload.ServerID,
			"status":    "failed",
		})

		return fmt.Errorf("failed to dispatch task: %w", err)
	}

	return nil
}
