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

const TypeInstallCron = "server:install_cron"

type InstallCronPayload struct {
	ServerID string  `json:"server_id"`
	CronID   string  `json:"cron_id"`
	UserID   *string `json:"user_id,omitempty"`
}

type InstallCronJob struct {
	ctx     *JobContext
	Payload InstallCronPayload
}

func (j *InstallCronJob) Handle(ctx context.Context) error {
	cron, err := j.ctx.Repo.FindCronByIDWithServer(ctx, j.Payload.CronID)
	if err != nil {
		return fmt.Errorf("failed to find cron: %w", err)
	}

	contents := cron.ToCronFileContents()

	task := tasks.UploadCron(tasks.UploadCronConfig{
		Path:     cron.Path(),
		Contents: contents,
		LogPath:  cron.GetLogPath(),
		User:     cron.User,
	})

	result, err := j.ctx.ForServer(cron.Server).RunTask(task).
		AsRoot().
		Dispatch(ctx)

	if err != nil {
		return fmt.Errorf("failed to upload cron file: %w", err)
	}

	if !result.IsSuccessful() {
		return fmt.Errorf("failed to upload cron file: %s", result.GetOutput())
	}

	if err := j.ctx.Repo.MarkCronInstalled(ctx, cron.ID); err != nil {
		return fmt.Errorf("failed to mark cron as installed: %w", err)
	}

	// Log activity
	logger := activity.New(j.ctx.DB).
		WithContext(ctx).
		UseLog("server").
		On(cron).
		WithEvent("installed")
	if j.Payload.UserID != nil {
		logger.CausedByUser(*j.Payload.UserID)
	}
	logger.Log("Cron job was installed")

	j.ctx.LogInfo("Cron installed successfully",
		"cron_id", cron.ID,
		"server_id", cron.ServerID,
		"command", cron.Command.String(),
	)

	j.ctx.BroadcastToServer(cron.ServerID, "cron.installed", map[string]any{
		"cron_id":   cron.ID,
		"server_id": cron.ServerID,
	})

	return nil
}

func (j *InstallCronJob) Failed(ctx context.Context, err error) {
	j.ctx.LogError(err, "Failed to install cron",
		"cron_id", j.Payload.CronID,
		"server_id", j.Payload.ServerID,
	)

	cron, findErr := j.ctx.Repo.FindCronByID(ctx, j.Payload.CronID)
	if findErr == nil && cron != nil {
		now := time.Now()
		j.ctx.DB.Model(cron).Updates(map[string]any{
			"installed_at":           nil,
			"installation_failed_at": &now,
		})
	}
}

func NewInstallCronJob(ctx *JobContext, payload InstallCronPayload) *InstallCronJob {
	return &InstallCronJob{
		ctx:     ctx,
		Payload: payload,
	}
}

func NewInstallCronTask(serverID, cronID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.NewTask(TypeInstallCron, InstallCronPayload{
		ServerID: serverID,
		CronID:   cronID,
		UserID:   userID,
	})
}
