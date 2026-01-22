package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	"github.com/kkz6/launch-go/internal/pkg/launch/activity"
)

const TypeUninstallCron = "server:uninstall_cron"

type UninstallCronPayload struct {
	ServerID string  `json:"server_id"`
	CronID   string  `json:"cron_id"`
	UserID   *string `json:"user_id,omitempty"`
}

// UninstallCronJob uninstalls a cron job from a server.
// Similar to Laravel's Modules\Server\Jobs\UninstallCron
type UninstallCronJob struct {
	Deps    *JobDeps
	Payload UninstallCronPayload

	cron *models.Cron
}

func NewUninstallCronJob(p UninstallCronPayload) pkgjobs.Handler {
	return &UninstallCronJob{Deps: deps, Payload: p}
}

// Handle processes the job
func (j *UninstallCronJob) Handle(ctx context.Context) error {
	var err error
	j.cron, err = j.Deps.Repos.Cron().FindByIDWithServer(ctx, j.Payload.CronID)
	if err != nil {
		return fmt.Errorf("failed to find cron: %w", err)
	}

	// Delete cron file from server
	task := tasks.DeleteFile(tasks.DeleteFileConfig{
		Path: j.cron.Path(),
	})

	result, err := j.Deps.RunTask(j.cron.Server, task).
		AsRoot().
		Dispatch(ctx)

	if err != nil {
		return fmt.Errorf("failed to delete cron file: %w", err)
	}

	if !result.IsSuccessful() {
		return fmt.Errorf("failed to delete cron file: %s", result.GetOutput())
	}

	// Log activity before deletion
	activity.RecordWithLogPtr(ctx, "server", "uninstalled", j.Payload.UserID, j.cron, "Cron job was uninstalled")

	// Delete the cron record
	if err := j.Deps.Repos.Cron().Delete(ctx, j.cron.ID); err != nil {
		return fmt.Errorf("failed to delete cron: %w", err)
	}

	j.Deps.Logger.Info().
		Str("cron_id", j.cron.ID).
		Str("server_id", j.cron.ServerID).
		Msg("Cron uninstalled successfully")

	// Broadcast event
	j.Deps.BroadcastServerEvent(j.cron.Server, "cron.uninstalled", map[string]any{
		"cron_id":   j.cron.ID,
		"server_id": j.cron.ServerID,
	})

	return nil
}

// Failed is called when the job fails after all retries
func (j *UninstallCronJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).
		Str("cron_id", j.Payload.CronID).
		Str("server_id", j.Payload.ServerID).
		Msg("Failed to uninstall cron")

	// Mark uninstallation as failed
	if markErr := j.Deps.Repos.Cron().MarkUninstallationFailed(ctx, j.Payload.CronID); markErr != nil {
		j.Deps.Logger.Error().Err(markErr).Msg("Failed to mark cron uninstallation as failed")
	}
}

// NewUninstallCronTask creates an asynq task for uninstalling a cron
// Uses TaskID for deduplication to prevent duplicate cron uninstallations
func NewUninstallCronTask(serverID, cronID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.TaskWithID(TypeUninstallCron,
		UninstallCronPayload{ServerID: serverID, CronID: cronID, UserID: userID},
		pkgjobs.Dedup("uninstall_cron", serverID, cronID),
	)
}
