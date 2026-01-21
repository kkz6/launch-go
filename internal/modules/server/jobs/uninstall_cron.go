package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/pkg/activity"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
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
	pkgjobs.BaseJob[*JobContext, UninstallCronPayload]
}

// Handle processes the job
func (j *UninstallCronJob) Handle(ctx context.Context) error {
	// Find the cron with server preloaded
	cron, err := j.Ctx.Repos.Cron().FindByIDWithServer(ctx, j.Payload.CronID)
	if err != nil {
		return fmt.Errorf("failed to find cron: %w", err)
	}

	// Delete cron file from server
	task := tasks.DeleteFile(tasks.DeleteFileConfig{
		Path: cron.Path(),
	})

	result, err := j.Ctx.ForServer(cron.Server).RunTask(task).
		AsRoot().
		Dispatch(ctx)

	if err != nil {
		return fmt.Errorf("failed to delete cron file: %w", err)
	}

	if !result.IsSuccessful() {
		return fmt.Errorf("failed to delete cron file: %s", result.GetOutput())
	}

	// Log activity before deletion
	activity.LogWithLogPtr(ctx, j.Ctx.DB, "server", "uninstalled", j.Payload.UserID, cron, "Cron job was uninstalled")

	// Delete the cron record
	if err := j.Ctx.Repos.Cron().Delete(ctx, cron.ID); err != nil {
		return fmt.Errorf("failed to delete cron: %w", err)
	}

	j.Ctx.LogInfo("Cron uninstalled successfully",
		"cron_id", cron.ID,
		"server_id", cron.ServerID,
	)

	// Broadcast event
	j.Ctx.BroadcastServerEvent(cron.Server, "cron.uninstalled", map[string]any{
		"cron_id":   cron.ID,
		"server_id": cron.ServerID,
	})

	return nil
}

// Failed is called when the job fails after all retries
func (j *UninstallCronJob) Failed(ctx context.Context, err error) {
	j.Ctx.LogError(err, "Failed to uninstall cron",
		"cron_id", j.Payload.CronID,
		"server_id", j.Payload.ServerID,
	)

	// Mark uninstallation as failed
	cron, findErr := j.Ctx.Repos.Cron().FindByID(ctx, j.Payload.CronID)
	if findErr == nil && cron != nil {
		now := time.Now()
		j.Ctx.DB.Model(cron).Updates(map[string]any{
			"uninstallation_requested_at": nil,
			"uninstallation_failed_at":    &now,
		})
	}
}

func NewUninstallCronJob(ctx *JobContext, payload UninstallCronPayload) *UninstallCronJob {
	return &UninstallCronJob{
		BaseJob: pkgjobs.NewBaseJob(ctx, payload),
	}
}

// NewUninstallCronTask creates an asynq task for uninstalling a cron
// Uses TaskID for deduplication to prevent duplicate cron uninstallations
func NewUninstallCronTask(serverID, cronID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.NewTask(TypeUninstallCron, UninstallCronPayload{
		ServerID: serverID,
		CronID:   cronID,
		UserID:   userID,
	}, asynq.TaskID(fmt.Sprintf("uninstall_cron:%s:%s", serverID, cronID)))
}
