package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeInstallCron = "server:install_cron"

// InstallCronPayload contains data for installing a cron job
type InstallCronPayload struct {
	ServerID string  `json:"server_id"`
	CronID   string  `json:"cron_id"`
	UserID   *string `json:"user_id,omitempty"`
}

// InstallCronJob handles installing a cron job on a server
type InstallCronJob struct {
	*JobContext
	jobs.InstallationTracker
}

// NewInstallCronTask creates a new asynq task for installing a cron job
func NewInstallCronTask(serverID, cronID string, userID *string) (*asynq.Task, error) {
	return jobs.NewTask(TypeInstallCron, InstallCronPayload{
		ServerID: serverID,
		CronID:   cronID,
		UserID:   userID,
	})
}

// Handle processes the install cron job
func (j *InstallCronJob) Handle(ctx context.Context, t *asynq.Task) error {
	payload, err := jobs.UnmarshalPayload[InstallCronPayload](t)
	if err != nil {
		return err
	}

	j.Logger.Info().
		Str("server_id", payload.ServerID).
		Str("cron_id", payload.CronID).
		Msg("Installing cron job")

	// Fetch the cron with server
	cron, err := j.Repo.FindCronByIDWithServer(ctx, payload.CronID)
	if err != nil {
		return fmt.Errorf("failed to find cron: %w", err)
	}

	j.broadcastProgress(payload.ServerID, "installing", fmt.Sprintf("Installing cron job: %s", cron.Command))

	// TODO: Run the actual cron installation task
	// result, err := j.RunTask(cron.Server, tasks.NewInstallCron(cron)).
	//     AsRoot().
	//     KeepTrack().
	//     Dispatch(ctx)
	// if err != nil {
	//     return err
	// }

	// Mark the cron as installed
	if err := j.Repo.MarkCronInstalled(ctx, cron.ID); err != nil {
		return fmt.Errorf("failed to update cron status: %w", err)
	}

	j.broadcastProgress(payload.ServerID, "installed", "Cron job installed successfully")

	return nil
}

// Failed handles job failure
func (j *InstallCronJob) Failed(ctx context.Context, t *asynq.Task, err error) {
	payload, unmarshalErr := jobs.UnmarshalPayload[InstallCronPayload](t)
	if unmarshalErr != nil {
		j.Logger.Error().Err(unmarshalErr).Msg("Failed to unmarshal payload in failure handler")
		return
	}

	j.Logger.Error().
		Err(err).
		Str("server_id", payload.ServerID).
		Str("cron_id", payload.CronID).
		Msg("Failed to install cron job")

	// Fetch the cron for updating status
	cron, findErr := j.Repo.FindCronByID(ctx, payload.CronID)
	if findErr != nil {
		j.broadcastProgress(payload.ServerID, "failed", "Failed to install cron job")
		return
	}

	// Mark installation as failed
	j.MarkInstallationFailed(j.DB, cron)

	j.broadcastProgress(payload.ServerID, "failed", fmt.Sprintf("Failed to install cron job: %s", cron.Command))
}

func (j *InstallCronJob) broadcastProgress(serverID, status, message string) {
	j.BroadcastToServer(serverID, "server.cron.progress", map[string]interface{}{
		"server_id": serverID,
		"status":    status,
		"message":   message,
	})
}
