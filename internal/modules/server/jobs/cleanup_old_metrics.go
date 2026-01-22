package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/types"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeCleanupOldMetrics = "server:cleanup_old_metrics"

// MetricsRetentionDays is the number of days to retain metrics data
const MetricsRetentionDays = 7

// CleanupOldMetricsPayload holds data for the cleanup job
type CleanupOldMetricsPayload struct{}

// CleanupOldMetricsJob cleans up metrics older than the retention period
type CleanupOldMetricsJob struct {
	Deps    *JobDeps
	Payload CleanupOldMetricsPayload
}

func NewCleanupOldMetricsJob(p CleanupOldMetricsPayload) pkgjobs.Handler {
	return &CleanupOldMetricsJob{Deps: deps, Payload: p}
}

func (j *CleanupOldMetricsJob) Handle(ctx context.Context) error {
	cutoff := time.Now().AddDate(0, 0, -MetricsRetentionDays)

	j.Deps.Logger.Info().
		Int("retention_days", MetricsRetentionDays).
		Str("cutoff_time", cutoff.Format(time.RFC3339)).
		Msg("starting metrics cleanup")

	deleted, err := j.Deps.Repos.Metric().DeleteOlderThan(ctx, cutoff)
	if err != nil {
		return fmt.Errorf("delete old metrics: %w", err)
	}

	j.Deps.Logger.Info().
		Int64("deleted_count", deleted).
		Str("cutoff_time", cutoff.Format(time.RFC3339)).
		Msg("metrics cleanup completed")

	return nil
}

func (j *CleanupOldMetricsJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).
		Msg("failed to cleanup old metrics")
}

func NewCleanupOldMetricsTask() (*asynq.Task, error) {
	return pkgjobs.Task(TypeCleanupOldMetrics, CleanupOldMetricsPayload{})
}

// CleanupOldMetricsCronSpec returns the cron spec for the cleanup job
// Runs daily at 2:00 AM
func CleanupOldMetricsCronSpec() string {
	return types.CronDaily2AM.Expression()
}
