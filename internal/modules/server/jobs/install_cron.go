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
	pkgjobs.BaseJob[*JobContext, InstallCronPayload]
}

func (j *InstallCronJob) Handle(ctx context.Context) error {
	cron, err := j.Ctx.Repos().Cron().FindByIDWithServer(ctx, j.Payload.CronID)
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

	result, err := j.Ctx.ForServer(cron.Server).RunTask(task).
		AsRoot().
		Dispatch(ctx)

	if err != nil {
		return fmt.Errorf("failed to upload cron file: %w", err)
	}

	if !result.IsSuccessful() {
		return fmt.Errorf("failed to upload cron file: %s", result.GetOutput())
	}

	if err := j.Ctx.Repos().Cron().MarkInstalled(ctx, cron.ID); err != nil {
		return fmt.Errorf("failed to mark cron as installed: %w", err)
	}

	// Log activity
	activity.LogWithLogPtr(ctx, j.Ctx.DB(), "server", "installed", j.Payload.UserID, cron, "Cron job was installed")

	j.Ctx.LogInfo("Cron installed successfully",
		"cron_id", cron.ID,
		"server_id", cron.ServerID,
		"command", cron.Command.String(),
	)

	j.Ctx.BroadcastServerEvent(cron.Server, "cron.installed", map[string]any{
		"cron_id":   cron.ID,
		"server_id": cron.ServerID,
	})

	return nil
}

func (j *InstallCronJob) Failed(ctx context.Context, err error) {
	j.Ctx.LogError(err, "Failed to install cron",
		"cron_id", j.Payload.CronID,
		"server_id", j.Payload.ServerID,
	)

	cron, findErr := j.Ctx.Repos().Cron().FindByID(ctx, j.Payload.CronID)
	if findErr == nil && cron != nil {
		now := time.Now()
		j.Ctx.DB().Model(cron).Updates(map[string]any{
			"installed_at":           nil,
			"installation_failed_at": &now,
		})
	}
}

func NewInstallCronJob(ctx *JobContext, payload InstallCronPayload) *InstallCronJob {
	return &InstallCronJob{
		BaseJob: pkgjobs.NewBaseJob(ctx, payload),
	}
}

// NewInstallCronTask creates an asynq task for installing a cron job
// Uses TaskID for deduplication to prevent duplicate cron installations
func NewInstallCronTask(serverID, cronID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.NewTask(TypeInstallCron, InstallCronPayload{
		ServerID: serverID,
		CronID:   cronID,
		UserID:   userID,
	}, asynq.TaskID(fmt.Sprintf("install_cron:%s:%s", serverID, cronID)))
}
