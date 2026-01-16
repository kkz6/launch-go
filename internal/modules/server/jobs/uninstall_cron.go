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
	ctx     *JobContext
	Payload UninstallCronPayload
}

// Handle processes the job
func (j *UninstallCronJob) Handle(ctx context.Context) error {
	// Find the cron with server preloaded
	cron, err := j.ctx.Repo.FindCronByIDWithServer(ctx, j.Payload.CronID)
	if err != nil {
		return fmt.Errorf("failed to find cron: %w", err)
	}

	// Delete cron file from server
	task := tasks.DeleteFile(tasks.DeleteFileConfig{
		Path: cron.Path(),
	})

	result, err := j.ctx.ForServer(cron.Server).RunTask(task).
		AsRoot().
		Dispatch(ctx)

	if err != nil {
		return fmt.Errorf("failed to delete cron file: %w", err)
	}

	if !result.IsSuccessful() {
		return fmt.Errorf("failed to delete cron file: %s", result.GetOutput())
	}

	// Log activity before deletion
	logger := activity.New(j.ctx.DB).
		WithContext(ctx).
		UseLog("server").
		On(cron).
		WithEvent("uninstalled")
	if j.Payload.UserID != nil {
		logger.CausedByUser(*j.Payload.UserID)
	}
	logger.Log("Cron job was uninstalled")

	// Delete the cron record
	if err := j.ctx.Repo.DeleteCron(ctx, cron.ID); err != nil {
		return fmt.Errorf("failed to delete cron: %w", err)
	}

	j.ctx.LogInfo("Cron uninstalled successfully",
		"cron_id", cron.ID,
		"server_id", cron.ServerID,
	)

	// Broadcast event
	j.ctx.BroadcastToServer(cron.ServerID, "cron.uninstalled", map[string]any{
		"cron_id":   cron.ID,
		"server_id": cron.ServerID,
	})

	return nil
}

// Failed is called when the job fails after all retries
func (j *UninstallCronJob) Failed(ctx context.Context, err error) {
	j.ctx.LogError(err, "Failed to uninstall cron",
		"cron_id", j.Payload.CronID,
		"server_id", j.Payload.ServerID,
	)

	// Mark uninstallation as failed
	cron, findErr := j.ctx.Repo.FindCronByID(ctx, j.Payload.CronID)
	if findErr == nil && cron != nil {
		now := time.Now()
		j.ctx.DB.Model(cron).Updates(map[string]any{
			"uninstallation_requested_at": nil,
			"uninstallation_failed_at":    &now,
		})
	}
}

func NewUninstallCronJob(ctx *JobContext, payload UninstallCronPayload) *UninstallCronJob {
	return &UninstallCronJob{
		ctx:     ctx,
		Payload: payload,
	}
}

// NewUninstallCronTask creates an asynq task for uninstalling a cron
func NewUninstallCronTask(serverID, cronID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.NewTask(TypeUninstallCron, UninstallCronPayload{
		ServerID: serverID,
		CronID:   cronID,
		UserID:   userID,
	})
}
