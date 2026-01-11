package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/enums"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeRemoveService = "server:remove_service"

// RemoveServicePayload contains data for removing a service from a server
type RemoveServicePayload struct {
	ServerID  string              `json:"server_id"`
	Software  enums.Software      `json:"software"`
	Operation enums.ServiceOption `json:"operation"`
}

// RemoveServiceJob handles removing a service from a server
type RemoveServiceJob struct {
	*JobContext
	jobs.UninstallationTracker
}

// NewRemoveServiceTask creates a new asynq task for removing a service
func NewRemoveServiceTask(serverID string, software enums.Software, operation enums.ServiceOption) (*asynq.Task, error) {
	return jobs.NewTask(TypeRemoveService, RemoveServicePayload{
		ServerID:  serverID,
		Software:  software,
		Operation: operation,
	})
}

// Handle processes the remove service job
func (j *RemoveServiceJob) Handle(ctx context.Context, t *asynq.Task) error {
	payload, err := jobs.UnmarshalPayload[RemoveServicePayload](t)
	if err != nil {
		return err
	}

	j.Logger.Info().
		Str("server_id", payload.ServerID).
		Str("software", string(payload.Software)).
		Str("operation", string(payload.Operation)).
		Msg("Removing service from server")

	// Fetch the server
	server, err := j.FindServer(ctx, payload.ServerID)
	if err != nil {
		return err
	}

	j.broadcastProgress(payload.ServerID, "removing", fmt.Sprintf("Removing %s...", payload.Software.Label()))

	// TODO: Run the actual service removal task
	// _, err = j.RunTask(server, tasks.NewRemoveService(payload.Software)).
	//     AsRoot().
	//     Dispatch(ctx)
	// if err != nil {
	//     return err
	// }

	_ = server // use server when implementing task execution

	// Delete the service record
	if err := j.DB.Where("server_id = ? AND software = ?", payload.ServerID, payload.Software).
		Delete(&models.InstalledService{}).Error; err != nil {
		return fmt.Errorf("failed to delete service record: %w", err)
	}

	j.broadcastProgress(payload.ServerID, "removed", fmt.Sprintf("%s removed successfully", payload.Software.Label()))

	return nil
}

// Failed handles job failure
func (j *RemoveServiceJob) Failed(ctx context.Context, t *asynq.Task, err error) {
	payload, unmarshalErr := jobs.UnmarshalPayload[RemoveServicePayload](t)
	if unmarshalErr != nil {
		j.Logger.Error().Err(unmarshalErr).Msg("Failed to unmarshal payload in failure handler")
		return
	}

	j.Logger.Error().
		Err(err).
		Str("server_id", payload.ServerID).
		Str("software", string(payload.Software)).
		Str("operation", string(payload.Operation)).
		Msg("Failed to remove service")

	j.broadcastProgress(payload.ServerID, "failed", fmt.Sprintf("Failed to remove %s", payload.Software.Label()))
}

func (j *RemoveServiceJob) broadcastProgress(serverID, status, message string) {
	j.BroadcastToServer(serverID, "server.service.progress", map[string]interface{}{
		"server_id": serverID,
		"status":    status,
		"message":   message,
	})
}
