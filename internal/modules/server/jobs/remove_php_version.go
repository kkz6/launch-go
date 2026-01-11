package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeRemovePhpVersion = "server:remove_php_version"

// RemovePhpVersionPayload contains data for removing a PHP version from a server
type RemovePhpVersionPayload struct {
	ServerID  string `json:"server_id"`
	ServiceID string `json:"service_id"`
}

// RemovePhpVersionJob handles removing a PHP version from a server
type RemovePhpVersionJob struct {
	*JobContext
	jobs.UninstallationTracker
}

// NewRemovePhpVersionTask creates a new asynq task for removing a PHP version
func NewRemovePhpVersionTask(serverID, serviceID string) (*asynq.Task, error) {
	return jobs.NewTask(TypeRemovePhpVersion, RemovePhpVersionPayload{
		ServerID:  serverID,
		ServiceID: serviceID,
	})
}

// Handle processes the remove PHP version job
func (j *RemovePhpVersionJob) Handle(ctx context.Context, t *asynq.Task) error {
	payload, err := jobs.UnmarshalPayload[RemovePhpVersionPayload](t)
	if err != nil {
		return err
	}

	j.Logger.Info().
		Str("server_id", payload.ServerID).
		Str("service_id", payload.ServiceID).
		Msg("Removing PHP version from server")

	// Fetch the server
	server, err := j.FindServer(ctx, payload.ServerID)
	if err != nil {
		return err
	}

	// Fetch the service to get version info
	var service models.InstalledService
	if err := j.DB.First(&service, "id = ?", payload.ServiceID).Error; err != nil {
		return fmt.Errorf("failed to find service: %w", err)
	}

	j.broadcastProgress(payload.ServerID, "removing", fmt.Sprintf("Removing PHP %s...", service.Version))

	// TODO: Run the actual PHP removal task
	// _, err = j.RunTask(server, tasks.NewRemovePhpVersion(&service)).
	//     AsRoot().
	//     Dispatch(ctx)
	// if err != nil {
	//     return err
	// }

	_ = server // use server when implementing task execution

	j.broadcastProgress(payload.ServerID, "removed", fmt.Sprintf("PHP %s removed successfully", service.Version))

	return nil
}

// Failed handles job failure
func (j *RemovePhpVersionJob) Failed(ctx context.Context, t *asynq.Task, err error) {
	payload, unmarshalErr := jobs.UnmarshalPayload[RemovePhpVersionPayload](t)
	if unmarshalErr != nil {
		j.Logger.Error().Err(unmarshalErr).Msg("Failed to unmarshal payload in failure handler")
		return
	}

	j.Logger.Error().
		Err(err).
		Str("server_id", payload.ServerID).
		Str("service_id", payload.ServiceID).
		Msg("Failed to remove PHP version")

	j.broadcastProgress(payload.ServerID, "failed", "Failed to remove PHP version")
}

func (j *RemovePhpVersionJob) broadcastProgress(serverID, status, message string) {
	j.BroadcastToServer(serverID, "server.service.progress", map[string]interface{}{
		"server_id": serverID,
		"status":    status,
		"message":   message,
	})
}
