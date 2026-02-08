package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/types"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeSyncAllDaemons = "server:sync_all_daemons"

// SyncAllDaemonsPayload is empty - this job iterates all eligible servers
type SyncAllDaemonsPayload struct{}

// SyncAllDaemonsJob finds all connected servers with LaunchAgent
// and dispatches individual SyncDaemons jobs for each.
type SyncAllDaemonsJob struct {
	Deps    *JobDeps
	Payload SyncAllDaemonsPayload
}

func NewSyncAllDaemonsJob(p SyncAllDaemonsPayload) pkgjobs.Handler {
	return &SyncAllDaemonsJob{Deps: deps, Payload: p}
}

func (j *SyncAllDaemonsJob) Handle(ctx context.Context) error {
	var serverIDs []string

	err := j.Deps.DB.WithContext(ctx).
		Model(&models.Server{}).
		Joins("JOIN services ON services.server_id = servers.id AND services.type = ?", types.ServiceTypeLaunchAgent).
		Where("servers.connected = ?", true).
		Where("servers.archived_at IS NULL").
		Pluck("servers.id", &serverIDs).Error
	if err != nil {
		return fmt.Errorf("find connected servers with launch agent: %w", err)
	}

	if len(serverIDs) == 0 {
		j.Deps.Logger.Info().Msg("no connected servers with launch agent to sync daemons")
		return nil
	}

	dispatched := 0
	for _, serverID := range serverIDs {
		task, err := NewSyncDaemonsTask(serverID, nil)
		if err != nil {
			j.Deps.Logger.Error().Err(err).Str("server_id", serverID).Msg("failed to create sync daemons task")
			continue
		}

		if err := j.Deps.DispatchTask(task, asynq.TaskID(pkgjobs.Dedup("sync_daemons", serverID))); err != nil {
			j.Deps.Logger.Error().Err(err).Str("server_id", serverID).Msg("failed to dispatch sync daemons task")
			continue
		}

		dispatched++
	}

	j.Deps.Logger.Info().
		Int("total_servers", len(serverIDs)).
		Int("dispatched", dispatched).
		Msg("sync all daemons completed")

	return nil
}

func (j *SyncAllDaemonsJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).Msg("sync all daemons job failed")
}

func NewSyncAllDaemonsTask() (*asynq.Task, error) {
	return pkgjobs.Task(TypeSyncAllDaemons, SyncAllDaemonsPayload{},
		asynq.TaskID(pkgjobs.Dedup("sync_all_daemons", time.Now().Format("2006-01-02"))),
	)
}
