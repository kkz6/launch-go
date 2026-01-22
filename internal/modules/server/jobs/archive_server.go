package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	"github.com/kkz6/launch-go/internal/pkg/launch/activity"
)

const TypeArchiveServer = "server:archive"

type ArchiveServerPayload struct {
	ServerID string  `json:"server_id"`
	UserID   *string `json:"user_id,omitempty"`
}

// ArchiveServerJob archives a server (soft delete).
// Similar to Laravel's Modules\Server\Jobs\ArchiveServer
type ArchiveServerJob struct {
	Deps    *JobDeps
	Payload ArchiveServerPayload

	server *models.Server
}

func NewArchiveServerJob(p ArchiveServerPayload) pkgjobs.Handler {
	return &ArchiveServerJob{Deps: deps, Payload: p}
}

func (j *ArchiveServerJob) Handle(ctx context.Context) error {
	var err error

	j.server, err = j.Deps.Repos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("find server: %w", err)
	}

	if err := j.Deps.Repos.Server().Archive(ctx, j.server.ID); err != nil {
		return fmt.Errorf("archive server: %w", err)
	}

	userID := ""
	if j.Payload.UserID != nil {
		userID = *j.Payload.UserID
	}
	activity.RecordEvent(ctx, "archived", userID, j.server, "Server was archived")

	j.Deps.Logger.Info().
		Str("server_id", j.server.ID).
		Str("server_name", j.server.Name).
		Msg("server archived successfully")

	j.Deps.BroadcastServerEvent(j.server, "server.archived", map[string]any{
		"server_id": j.server.ID,
	})

	return nil
}

func (j *ArchiveServerJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).
		Str("server_id", j.Payload.ServerID).
		Msg("failed to archive server")
}

func NewArchiveServerTask(serverID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.TaskWithID(TypeArchiveServer,
		ArchiveServerPayload{ServerID: serverID, UserID: userID},
		pkgjobs.Dedup("archive_server", serverID),
	)
}
