package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeUpdateConnectivity = "server:update_connectivity"

type UpdateConnectivityPayload struct {
	ServerID string `json:"server_id"`
}

// UpdateConnectivityJob checks and updates the connectivity status of a server.
// Similar to Laravel's Modules\Server\Jobs\UpdateConnectivity
type UpdateConnectivityJob struct {
	Deps    *JobDeps
	Payload UpdateConnectivityPayload

	server *models.Server
}

func NewUpdateConnectivityJob(p UpdateConnectivityPayload) pkgjobs.Handler {
	return &UpdateConnectivityJob{Deps: deps, Payload: p}
}

// Handle processes the job
func (j *UpdateConnectivityJob) Handle(ctx context.Context) error {
	var err error
	j.server, err = j.Deps.Repos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	// Run a simple connectivity check (whoami)
	task := tasks.Whoami()

	result, err := j.Deps.RunTask(j.server, task).
		AsRoot().
		Dispatch(ctx)

	isConnected := err == nil && result != nil && result.IsSuccessful()

	// Update server connectivity status
	if err := j.Deps.Repos.Server().UpdateFields(ctx, j.server.ID, map[string]any{
		"connected": isConnected,
	}); err != nil {
		return fmt.Errorf("failed to update connectivity status: %w", err)
	}

	j.Deps.Logger.Info().
		Str("server_id", j.server.ID).
		Bool("is_connected", isConnected).
		Msg("Server connectivity updated")

	// Broadcast event
	j.Deps.BroadcastServerEvent(j.server, "server.connectivity", map[string]any{
		"server_id":    j.server.ID,
		"is_connected": isConnected,
	})

	return nil
}

// Failed is called when the job fails after all retries
func (j *UpdateConnectivityJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).
		Str("server_id", j.Payload.ServerID).
		Msg("Failed to update connectivity")

	// Mark server as disconnected on failure
	if updateErr := j.Deps.Repos.Server().UpdateFields(ctx, j.Payload.ServerID, map[string]any{
		"connected": false,
	}); updateErr != nil {
		j.Deps.Logger.Error().Err(updateErr).
			Str("server_id", j.Payload.ServerID).
			Msg("Failed to mark server disconnected after connectivity failure")
	}
}

// NewUpdateConnectivityTask creates an asynq task for updating server connectivity
func NewUpdateConnectivityTask(serverID string) (*asynq.Task, error) {
	return pkgjobs.Task(TypeUpdateConnectivity, UpdateConnectivityPayload{
		ServerID: serverID,
	})
}
