package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeCheckAllConnectivity = "server:check_all_connectivity"

// CheckAllConnectivityPayload is empty - this job iterates all eligible servers
type CheckAllConnectivityPayload struct{}

// CheckAllConnectivityJob finds all servers that haven't been updated in the
// last week and dispatches individual UpdateConnectivity jobs for each.
type CheckAllConnectivityJob struct {
	Deps    *JobDeps
	Payload CheckAllConnectivityPayload
}

func NewCheckAllConnectivityJob(p CheckAllConnectivityPayload) pkgjobs.Handler {
	return &CheckAllConnectivityJob{Deps: deps, Payload: p}
}

func (j *CheckAllConnectivityJob) Handle(ctx context.Context) error {
	threshold := time.Now().AddDate(0, 0, -7)

	var serverIDs []string

	err := j.Deps.DB.WithContext(ctx).
		Model(&models.Server{}).
		Where("updated_at < ?", threshold).
		Where("archived_at IS NULL").
		Pluck("id", &serverIDs).Error
	if err != nil {
		return fmt.Errorf("find servers for connectivity check: %w", err)
	}

	if len(serverIDs) == 0 {
		j.Deps.Logger.Info().Msg("no servers need connectivity check")
		return nil
	}

	dispatched := 0
	for _, serverID := range serverIDs {
		task, err := NewUpdateConnectivityTask(serverID)
		if err != nil {
			j.Deps.Logger.Error().Err(err).Str("server_id", serverID).Msg("failed to create connectivity task")
			continue
		}

		if err := j.Deps.DispatchTask(task, asynq.TaskID(pkgjobs.Dedup("update_connectivity", serverID))); err != nil {
			j.Deps.Logger.Error().Err(err).Str("server_id", serverID).Msg("failed to dispatch connectivity task")
			continue
		}

		dispatched++
	}

	j.Deps.Logger.Info().
		Int("total_servers", len(serverIDs)).
		Int("dispatched", dispatched).
		Msg("check all connectivity completed")

	return nil
}

func (j *CheckAllConnectivityJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).Msg("check all connectivity job failed")
}

func NewCheckAllConnectivityTask() (*asynq.Task, error) {
	return pkgjobs.Task(TypeCheckAllConnectivity, CheckAllConnectivityPayload{},
		asynq.TaskID(pkgjobs.Dedup("check_all_connectivity", time.Now().Format("2006-01-02"))),
	)
}
