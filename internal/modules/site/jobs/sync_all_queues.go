package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/site/models"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeSyncAllQueues = "site:sync_all_queues"

// SyncAllQueuesPayload is empty - this job iterates all eligible sites
type SyncAllQueuesPayload struct{}

// SyncAllQueuesJob finds all sites that have queues on connected servers
// and dispatches individual SyncQueues jobs for each.
type SyncAllQueuesJob struct {
	Deps    *JobDeps
	Payload SyncAllQueuesPayload
}

func NewSyncAllQueuesJob(p SyncAllQueuesPayload) pkgjobs.Handler {
	return &SyncAllQueuesJob{Deps: deps, Payload: p}
}

func (j *SyncAllQueuesJob) Handle(ctx context.Context) error {
	// Find distinct (site_id, server_id) pairs from queues where the server is connected
	var results []struct {
		SiteID   string
		ServerID string
	}

	err := j.Deps.DB.WithContext(ctx).
		Model(&models.Queue{}).
		Select("DISTINCT queues.site_id, queues.server_id").
		Joins("JOIN servers ON servers.id = queues.server_id").
		Where("servers.connected = ?", true).
		Where("servers.archived_at IS NULL").
		Find(&results).Error
	if err != nil {
		return fmt.Errorf("find sites with queues on connected servers: %w", err)
	}

	if len(results) == 0 {
		j.Deps.Logger.Info().Msg("no sites with queues on connected servers to sync")
		return nil
	}

	dispatched := 0
	for _, r := range results {
		task, err := NewSyncQueuesTask(r.SiteID, r.ServerID, nil)
		if err != nil {
			j.Deps.Logger.Error().Err(err).
				Str("site_id", r.SiteID).
				Msg("failed to create sync queues task")
			continue
		}

		taskID := pkgjobs.Dedup("sync_queues", r.SiteID, time.Now().UTC().Format("2006-01-02"))
		if err := j.Deps.DispatchTask(task, asynq.TaskID(taskID), asynq.MaxRetry(2)); err != nil {
			j.Deps.Logger.Error().Err(err).
				Str("site_id", r.SiteID).
				Msg("failed to dispatch sync queues task")
			continue
		}

		dispatched++
	}

	j.Deps.Logger.Info().
		Int("total_sites", len(results)).
		Int("dispatched", dispatched).
		Msg("sync all queues completed")

	return nil
}

func (j *SyncAllQueuesJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).Msg("sync all queues job failed")
}

func NewSyncAllQueuesTask() (*asynq.Task, error) {
	return pkgjobs.Task(TypeSyncAllQueues, SyncAllQueuesPayload{}, asynq.MaxRetry(2))
}
