package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/enums"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeAddService = "server:add_service"

// AddServicePayload contains data for adding a service to a server
type AddServicePayload struct {
	ServerID  string         `json:"server_id"`
	ServiceID string         `json:"service_id"`
	Software  enums.Software `json:"software"`
}

// AddServiceJob handles adding a service to a server
type AddServiceJob struct {
	*JobContext
	jobs.InstallationTracker
}

// NewAddServiceTask creates a new asynq task for adding a service
func NewAddServiceTask(serverID, serviceID string, software enums.Software) (*asynq.Task, error) {
	return jobs.NewTask(TypeAddService, AddServicePayload{
		ServerID:  serverID,
		ServiceID: serviceID,
		Software:  software,
	})
}

// Handle processes the add service job
func (j *AddServiceJob) Handle(ctx context.Context, t *asynq.Task) error {
	payload, err := jobs.UnmarshalPayload[AddServicePayload](t)
	if err != nil {
		return err
	}

	j.Logger.Info().
		Str("server_id", payload.ServerID).
		Str("service_id", payload.ServiceID).
		Str("software", string(payload.Software)).
		Msg("Adding service to server")

	// Fetch the server
	server, err := j.FindServer(ctx, payload.ServerID)
	if err != nil {
		return err
	}

	// Fetch the service
	var service models.InstalledService
	if err := j.DB.First(&service, "id = ?", payload.ServiceID).Error; err != nil {
		return fmt.Errorf("failed to find service: %w", err)
	}

	j.broadcastProgress(payload.ServerID, "installing", fmt.Sprintf("Installing %s...", payload.Software.Label()))

	// TODO: Run the actual service installation task
	// result, err := j.RunTask(server, tasks.NewAddService(payload.Software)).
	//     AsRoot().
	//     KeepTrack().
	//     Dispatch(ctx)
	// if err != nil {
	//     return err
	// }
	// Update service with task ID: j.DB.Model(&service).Update("task_id", result.TaskModel.ID)

	_ = server // use server when implementing task execution

	j.broadcastProgress(payload.ServerID, "installed", fmt.Sprintf("%s installed successfully", payload.Software.Label()))

	return nil
}

// Failed handles job failure
func (j *AddServiceJob) Failed(ctx context.Context, t *asynq.Task, err error) {
	payload, unmarshalErr := jobs.UnmarshalPayload[AddServicePayload](t)
	if unmarshalErr != nil {
		j.Logger.Error().Err(unmarshalErr).Msg("Failed to unmarshal payload in failure handler")
		return
	}

	j.Logger.Error().
		Err(err).
		Str("server_id", payload.ServerID).
		Str("service_id", payload.ServiceID).
		Str("software", string(payload.Software)).
		Msg("Failed to add service")

	// Update service status to failed
	j.DB.Model(&models.InstalledService{}).
		Where("id = ?", payload.ServiceID).
		Update("status", enums.ServiceStatusFailed)

	j.broadcastProgress(payload.ServerID, "failed", fmt.Sprintf("Failed to install %s", payload.Software.Label()))
}

func (j *AddServiceJob) broadcastProgress(serverID, status, message string) {
	j.BroadcastToServer(serverID, "server.service.progress", map[string]interface{}{
		"server_id": serverID,
		"status":    status,
		"message":   message,
	})
}
