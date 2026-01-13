package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/pkg/activity"
	"github.com/kkz6/launch-go/internal/pkg/jobs"
)

type InstallCronJob struct {
	ServerJobBase
	jobs.InstallationTracker
	Payload InstallCronPayload
}

func (j *InstallCronJob) Type() string {
	return TypeInstallCron
}

func (j *InstallCronJob) Handle(ctx context.Context) error {
	cron, err := j.Repo().FindCronByIDWithServer(ctx, j.Payload.CronID)
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

	result, err := j.RunTaskOnServer(cron.Server, task).
		AsRoot().
		Dispatch(ctx)

	if err != nil {
		return fmt.Errorf("failed to upload cron file: %w", err)
	}

	if !result.IsSuccessful() {
		return fmt.Errorf("failed to upload cron file: %s", result.GetOutput())
	}

	if err := j.Repo().MarkCronInstalled(ctx, cron.ID); err != nil {
		return fmt.Errorf("failed to mark cron as installed: %w", err)
	}

	// Log activity
	logger := activity.New(j.DB).
		WithContext(ctx).
		UseLog("server").
		On(cron).
		WithEvent("installed")
	if j.Payload.UserID != nil {
		logger.CausedByUser(*j.Payload.UserID)
	}
	logger.Log("Cron job was installed")

	j.LogInfo("Cron installed successfully",
		"cron_id", cron.ID,
		"server_id", cron.ServerID,
		"command", cron.Command.String(),
	)

	j.BroadcastServerEvent(cron.ServerID, "cron.installed", map[string]any{
		"cron_id":   cron.ID,
		"server_id": cron.ServerID,
	})

	return nil
}

func (j *InstallCronJob) Failed(ctx context.Context, err error) {
	j.LogError(err, "Failed to install cron",
		"cron_id", j.Payload.CronID,
		"server_id", j.Payload.ServerID,
	)

	cron, findErr := j.Repo().FindCronByID(ctx, j.Payload.CronID)
	if findErr == nil && cron != nil {
		j.MarkInstallationFailed(j.DB, cron)
	}
}

func NewInstallCronTask(serverID, cronID string, userID *string) (*asynq.Task, error) {
	return jobs.NewTask(TypeInstallCron, InstallCronPayload{
		ServerID: serverID,
		CronID:   cronID,
		UserID:   userID,
	})
}
