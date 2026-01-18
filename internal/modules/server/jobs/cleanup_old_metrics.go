package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/hibiken/asynq"

	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeCleanupOldMetrics = "server:cleanup_old_metrics"

// MetricsRetentionDays is the number of days to retain metrics data
const MetricsRetentionDays = 7

// CleanupOldMetricsPayload holds data for the cleanup job
type CleanupOldMetricsPayload struct{}

// CleanupOldMetricsJob cleans up metrics older than the retention period
type CleanupOldMetricsJob struct {
	ctx     *JobContext
	Payload CleanupOldMetricsPayload
}

// NewCleanupOldMetricsJob creates a new CleanupOldMetricsJob
func NewCleanupOldMetricsJob(ctx *JobContext, payload CleanupOldMetricsPayload) *CleanupOldMetricsJob {
	return &CleanupOldMetricsJob{
		ctx:     ctx,
		Payload: payload,
	}
}

// Handle executes the cleanup job
func (j *CleanupOldMetricsJob) Handle(ctx context.Context) error {
	cutoff := time.Now().AddDate(0, 0, -MetricsRetentionDays)

	j.ctx.LogInfo("Starting metrics cleanup",
		"retention_days", MetricsRetentionDays,
		"cutoff_time", cutoff.Format(time.RFC3339),
	)

	deleted, err := j.ctx.Repos.Metric().DeleteOlderThan(ctx, cutoff)
	if err != nil {
		return fmt.Errorf("failed to delete old metrics: %w", err)
	}

	j.ctx.LogInfo("Metrics cleanup completed",
		"deleted_count", deleted,
		"cutoff_time", cutoff.Format(time.RFC3339),
	)

	return nil
}

// Failed handles job failure
func (j *CleanupOldMetricsJob) Failed(ctx context.Context, err error) {
	j.ctx.LogError(err, "Failed to cleanup old metrics")
}

// NewCleanupOldMetricsTask creates an asynq task for cleaning up old metrics
func NewCleanupOldMetricsTask() (*asynq.Task, error) {
	return pkgjobs.NewTask(TypeCleanupOldMetrics, CleanupOldMetricsPayload{})
}

// CleanupOldMetricsCronSpec returns the cron spec for the cleanup job
// Runs daily at 2:00 AM
func CleanupOldMetricsCronSpec() string {
	return "0 2 * * *"
}
