package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	"github.com/kkz6/launch-go/internal/pkg/launch/activity"
)

const TypeUnarchiveServer = "server:unarchive"

type UnarchiveServerPayload struct {
	ServerID string  `json:"server_id"`
	UserID   *string `json:"user_id,omitempty"`
}

// UnarchiveServerJob unarchives a server (restore from soft delete).
// Similar to Laravel's Modules\Server\Jobs\UnarchiveServer
type UnarchiveServerJob struct {
	Deps    *JobDeps
	Payload UnarchiveServerPayload

	server *models.Server
}

func NewUnarchiveServerJob(p UnarchiveServerPayload) pkgjobs.Handler {
	return &UnarchiveServerJob{Deps: deps, Payload: p}
}

// Handle processes the job
func (j *UnarchiveServerJob) Handle(ctx context.Context) error {
	var err error
	j.server, err = j.Deps.Repos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	// Unarchive the server
	if err := j.Deps.Repos.Server().Unarchive(ctx, j.server.ID); err != nil {
		return fmt.Errorf("failed to unarchive server: %w", err)
	}

	// Log activity
	activity.RecordWithLogPtr(ctx, "server", "unarchived", j.Payload.UserID, j.server, "Server was unarchived")

	j.Deps.Logger.Info().
		Str("server_id", j.server.ID).
		Str("server_name", j.server.Name).
		Msg("Server unarchived successfully")

	// Broadcast event
	j.Deps.BroadcastServerEvent(j.server, "server.unarchived", map[string]any{
		"server_id": j.server.ID,
	})

	return nil
}

// Failed is called when the job fails after all retries
func (j *UnarchiveServerJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).
		Str("server_id", j.Payload.ServerID).
		Msg("Failed to unarchive server")
}

// NewUnarchiveServerTask creates an asynq task for unarchiving a server
// Uses TaskID for deduplication to prevent duplicate unarchive operations
func NewUnarchiveServerTask(serverID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.TaskWithID(TypeUnarchiveServer,
		UnarchiveServerPayload{ServerID: serverID, UserID: userID},
		pkgjobs.Dedup("unarchive_server", serverID),
	)
}
