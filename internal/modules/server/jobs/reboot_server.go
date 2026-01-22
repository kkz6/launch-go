package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	"github.com/kkz6/launch-go/internal/pkg/launch/activity"
)

const TypeRebootServer = "server:reboot"

type RebootServerPayload struct {
	ServerID string  `json:"server_id"`
	UserID   *string `json:"user_id,omitempty"`
}

type RebootServerJob struct {
	Deps    *JobDeps
	Payload RebootServerPayload

	server *models.Server
}

func NewRebootServerJob(p RebootServerPayload) pkgjobs.Handler {
	return &RebootServerJob{Deps: deps, Payload: p}
}

func (j *RebootServerJob) Handle(ctx context.Context) error {
	var err error
	j.server, err = j.Deps.Repos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("find server: %w", err)
	}

	task := tasks.RebootServer()

	_, err = j.Deps.RunTask(j.server, task).
		AsRoot().
		Dispatch(ctx)

	if err != nil {
		j.Deps.Logger.Info().
			Str("server_id", j.server.ID).
			Msg("reboot command sent, connection dropped as expected")
	}

	activity.RecordWithLogPtr(ctx, "server", "rebooted", j.Payload.UserID, j.server, "Server reboot was initiated")

	j.Deps.Logger.Info().
		Str("server_id", j.server.ID).
		Str("server_name", j.server.Name).
		Msg("server reboot initiated")

	j.Deps.BroadcastServerEvent(j.server, "server.rebooting", map[string]any{
		"server_id": j.server.ID,
	})

	return nil
}

func (j *RebootServerJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).
		Str("server_id", j.Payload.ServerID).
		Msg("failed to reboot server")
}

func NewRebootServerTask(serverID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.TaskWithID(TypeRebootServer,
		RebootServerPayload{ServerID: serverID, UserID: userID},
		pkgjobs.Dedup("reboot", serverID),
	)
}
