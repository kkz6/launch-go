package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/modules/server/types"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	"github.com/kkz6/launch-go/internal/pkg/launch/activity"
)

const TypeRemoveService = "server:remove_service"

type RemoveServicePayload struct {
	ServerID  string  `json:"server_id"`
	ServiceID string  `json:"service_id"`
	UserID    *string `json:"user_id,omitempty"`
}

// RemoveServiceJob removes a service from a server.
// Similar to Laravel's Modules\Server\Jobs\UninstallService
type RemoveServiceJob struct {
	Deps    *JobDeps
	Payload RemoveServicePayload

	server  *models.Server
	service *models.InstalledService
}

func NewRemoveServiceJob(p RemoveServicePayload) pkgjobs.Handler {
	return &RemoveServiceJob{Deps: deps, Payload: p}
}

// Handle processes the job
func (j *RemoveServiceJob) Handle(ctx context.Context) error {
	var err error

	// Find the service with server
	j.service, err = j.Deps.Repos.Service().FindByID(ctx, j.Payload.ServiceID)
	if err != nil {
		return fmt.Errorf("failed to find service: %w", err)
	}

	// Find the server
	j.server, err = j.Deps.Repos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	// Mark service as uninstalling and broadcast status change
	if err := j.Deps.Repos.Service().UpdateStatus(ctx, j.service.ID, types.ServiceStatusUninstalling); err != nil {
		return fmt.Errorf("failed to update service status: %w", err)
	}

	j.Deps.BroadcastServerEvent(j.server, "service.status_changed", map[string]any{
		"service_id": j.service.ID,
		"server_id":  j.server.ID,
		"status":     string(types.ServiceStatusUninstalling),
	})

	// Create remove task using the software's remove template
	task := tasks.RemoveSoftware(j.service.GetSoftware())

	result, err := j.Deps.RunTask(j.server, task).
		AsRoot().
		Dispatch(ctx)

	if err != nil {
		return fmt.Errorf("failed to remove service: %w", err)
	}

	if !result.IsSuccessful() {
		j.Deps.Logger.Error().
			Str("output", result.GetOutput()).
			Msg("service removal completed with errors")
	}

	// Log activity before deletion
	activity.RecordWithLogPtr(ctx, "server", "removed", j.Payload.UserID, j.service, "Service was removed")

	// Delete the service record
	if err := j.Deps.Repos.Service().Delete(ctx, j.service.ID); err != nil {
		return fmt.Errorf("failed to delete service record: %w", err)
	}

	j.Deps.Logger.Info().
		Str("service_id", j.service.ID).
		Str("server_id", j.server.ID).
		Msg("service removed successfully")

	// Broadcast event
	j.Deps.BroadcastServerEvent(j.server, "service.removed", map[string]any{
		"service_id": j.service.ID,
		"server_id":  j.server.ID,
	})

	return nil
}

// Failed is called when the job fails after all retries
func (j *RemoveServiceJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).
		Str("service_id", j.Payload.ServiceID).
		Str("server_id", j.Payload.ServerID).
		Msg("failed to remove service")

	// Mark removal as failed
	if markErr := j.Deps.Repos.Service().MarkRemovalFailed(ctx, j.Payload.ServiceID); markErr != nil {
		j.Deps.Logger.Error().Err(markErr).
			Msg("failed to mark service removal as failed")
	}
}

// NewRemoveServiceTask creates an asynq task for removing a service
// Uses TaskID for deduplication to prevent duplicate service removals
func NewRemoveServiceTask(serverID, serviceID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.TaskWithID(TypeRemoveService,
		RemoveServicePayload{
			ServerID:  serverID,
			ServiceID: serviceID,
			UserID:    userID,
		},
		pkgjobs.Dedup("remove_service", serverID, serviceID),
	)
}
