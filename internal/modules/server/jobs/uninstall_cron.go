package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeUninstallCron = "server:uninstall_cron"

// UninstallCronPayload contains data for uninstalling a cron job
type UninstallCronPayload struct {
	ServerID string  `json:"server_id"`
	CronID   string  `json:"cron_id"`
	UserID   *string `json:"user_id,omitempty"`
}

// UninstallCronJob handles uninstalling a cron job from a server
type UninstallCronJob struct {
	*JobContext
	jobs.UninstallationTracker
}

// NewUninstallCronTask creates a new asynq task for uninstalling a cron job
func NewUninstallCronTask(serverID, cronID string, userID *string) (*asynq.Task, error) {
	return jobs.NewTask(TypeUninstallCron, UninstallCronPayload{
		ServerID: serverID,
		CronID:   cronID,
		UserID:   userID,
	})
}

// Handle processes the uninstall cron job
func (j *UninstallCronJob) Handle(ctx context.Context, t *asynq.Task) error {
	payload, err := jobs.UnmarshalPayload[UninstallCronPayload](t)
	if err != nil {
		return err
	}

	j.Logger.Info().
		Str("server_id", payload.ServerID).
		Str("cron_id", payload.CronID).
		Msg("Uninstalling cron job")

	// Fetch the cron with server
	cron, err := j.Repo.FindCronByIDWithServer(ctx, payload.CronID)
	if err != nil {
		return fmt.Errorf("failed to find cron: %w", err)
	}

	j.broadcastProgress(payload.ServerID, "uninstalling", fmt.Sprintf("Uninstalling cron job: %s", cron.Command))

	// Uninstall the cron job from the server
	_, err = j.RunTask(cron.Server, tasks.NewUninstallCron(cron.Server, cron)).
		AsRoot().
		Throw().
		Dispatch(ctx)
	if err != nil {
		return fmt.Errorf("failed to uninstall cron: %w", err)
	}

	// Delete the cron record from database
	if err := j.Repo.DeleteCron(ctx, cron.ID); err != nil {
		return fmt.Errorf("failed to delete cron record: %w", err)
	}

	j.broadcastProgress(payload.ServerID, "uninstalled", "Cron job uninstalled successfully")

	return nil
}

// Failed handles job failure
func (j *UninstallCronJob) Failed(ctx context.Context, t *asynq.Task, err error) {
	payload, unmarshalErr := jobs.UnmarshalPayload[UninstallCronPayload](t)
	if unmarshalErr != nil {
		j.Logger.Error().Err(unmarshalErr).Msg("Failed to unmarshal payload in failure handler")
		return
	}

	j.Logger.Error().
		Err(err).
		Str("server_id", payload.ServerID).
		Str("cron_id", payload.CronID).
		Msg("Failed to uninstall cron job")

	// Fetch the cron for error message
	cron, findErr := j.Repo.FindCronByID(ctx, payload.CronID)
	if findErr != nil {
		j.broadcastProgress(payload.ServerID, "failed", "Failed to uninstall cron job")
		return
	}

	j.broadcastProgress(payload.ServerID, "failed", fmt.Sprintf("Failed to uninstall cron job: %s", cron.Command))
}

func (j *UninstallCronJob) broadcastProgress(serverID, status, message string) {
	j.BroadcastToServer(serverID, "server.cron.progress", map[string]interface{}{
		"server_id": serverID,
		"status":    status,
		"message":   message,
		"timestamp": time.Now().Format(time.RFC3339),
	})
}
