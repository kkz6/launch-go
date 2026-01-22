package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeCheckDaemonStatus = "server:check_daemon_status"

// CheckDaemonStatusPayload holds data for daemon status check
type CheckDaemonStatusPayload struct {
	ServerID string  `json:"server_id"`
	UserID   *string `json:"user_id,omitempty"`
}

// CheckDaemonStatusJob checks the status of all daemons on a server
// This is typically triggered by a scheduled command
type CheckDaemonStatusJob struct {
	Deps    *JobDeps
	Payload CheckDaemonStatusPayload

	server *models.Server
}

func NewCheckDaemonStatusJob(p CheckDaemonStatusPayload) pkgjobs.Handler {
	return &CheckDaemonStatusJob{Deps: deps, Payload: p}
}

// Handle executes the daemon status check job
// This job delegates to SyncDaemons for the actual work
func (j *CheckDaemonStatusJob) Handle(ctx context.Context) error {
	var err error

	j.server, err = j.Deps.Repos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("find server: %w", err)
	}

	j.Deps.Logger.Info().
		Str("server_id", j.server.ID).
		Msg("checking daemon status")

	// Create and execute the sync daemons job directly
	syncJob := &SyncDaemonsJob{
		Deps:    j.Deps,
		Payload: SyncDaemonsPayload{ServerID: j.Payload.ServerID, UserID: j.Payload.UserID},
	}

	if err := syncJob.Handle(ctx); err != nil {
		return fmt.Errorf("sync daemon status: %w", err)
	}

	j.Deps.Logger.Info().
		Str("server_id", j.server.ID).
		Msg("daemon status check completed")

	return nil
}

func (j *CheckDaemonStatusJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).
		Str("server_id", j.Payload.ServerID).
		Msg("daemon status check failed")
}

func NewCheckDaemonStatusTask(serverID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.Task(TypeCheckDaemonStatus, CheckDaemonStatusPayload{
		ServerID: serverID,
		UserID:   userID,
	})
}
