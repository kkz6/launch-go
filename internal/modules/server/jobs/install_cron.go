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

const TypeInstallCron = "server:install_cron"

type InstallCronPayload struct {
	ServerID string  `json:"server_id"`
	CronID   string  `json:"cron_id"`
	UserID   *string `json:"user_id,omitempty"`
}

type InstallCronJob struct {
	Deps    *JobDeps
	Payload InstallCronPayload

	cron *models.Cron
}

func NewInstallCronJob(p InstallCronPayload) pkgjobs.Handler {
	return &InstallCronJob{Deps: deps, Payload: p}
}

func (j *InstallCronJob) Handle(ctx context.Context) error {
	var err error
	j.cron, err = j.Deps.Repos.Cron().FindByIDWithServer(ctx, j.Payload.CronID)
	if err != nil {
		return fmt.Errorf("failed to find cron: %w", err)
	}

	contents := j.cron.ToCronFileContents()

	task := tasks.UploadCron(tasks.UploadCronConfig{
		Path:     j.cron.Path(),
		Contents: contents,
		LogPath:  j.cron.GetLogPath(),
		User:     j.cron.User,
	})

	result, err := j.Deps.RunTask(j.cron.Server, task).
		AsRoot().
		Dispatch(ctx)

	if err != nil {
		return fmt.Errorf("failed to upload cron file: %w", err)
	}

	if !result.IsSuccessful() {
		return fmt.Errorf("failed to upload cron file: %s", result.GetOutput())
	}

	if err := j.Deps.Repos.Cron().MarkAsInstalled(ctx, j.cron.ID); err != nil {
		return fmt.Errorf("failed to mark cron as installed: %w", err)
	}

	// Log activity
	activity.RecordWithLogPtr(ctx, "server", "installed", j.Payload.UserID, j.cron, "Cron job was installed")

	j.Deps.Logger.Info().
		Str("cron_id", j.cron.ID).
		Str("server_id", j.cron.ServerID).
		Str("command", j.cron.Command.String()).
		Msg("cron installed successfully")

	j.Deps.BroadcastServerEvent(j.cron.Server, "cron.installed", map[string]any{
		"cron_id":   j.cron.ID,
		"server_id": j.cron.ServerID,
	})

	return nil
}

func (j *InstallCronJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).
		Str("cron_id", j.Payload.CronID).
		Str("server_id", j.Payload.ServerID).
		Msg("failed to install cron")

	// Mark installation as failed
	if markErr := j.Deps.Repos.Cron().MarkInstallationFailed(ctx, j.Payload.CronID); markErr != nil {
		j.Deps.Logger.Error().Err(markErr).Msg("failed to mark cron installation as failed")
	}
}

// NewInstallCronTask creates an asynq task for installing a cron job
// Uses TaskID for deduplication to prevent duplicate cron installations
func NewInstallCronTask(serverID, cronID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.Task(TypeInstallCron, InstallCronPayload{
		ServerID: serverID,
		CronID:   cronID,
		UserID:   userID,
	}, asynq.TaskID(pkgjobs.Dedup("install_cron", serverID, cronID)))
}
