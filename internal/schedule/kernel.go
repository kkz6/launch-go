package schedule

import (
	serverjobs "github.com/kkz6/launch-go/internal/modules/server/jobs"
	sitejobs "github.com/kkz6/launch-go/internal/modules/site/jobs"
	"github.com/kkz6/launch-go/internal/pkg/queue"
)

// GetScheduledTasks returns all scheduled tasks for the application.
//
// This is the central place to define all cron jobs. Add new scheduled tasks here.
//
// Schedule helpers:
//   - EveryMinute(taskFn, opts...)   - Run every minute
//   - Every5Minutes(taskFn, opts...) - Run every 5 minutes
//   - Hourly(taskFn, opts...)        - Run every hour at :00
//   - Daily(taskFn, opts...)         - Run daily at midnight
//   - Weekly(taskFn, opts...)        - Run weekly on Sunday at midnight
//   - Monthly(taskFn, opts...)       - Run monthly on the 1st at midnight
//   - At(cron, taskFn, opts...)      - Run at custom cron schedule
//
// Task options:
//   - WithName("task-name")  - Set a name for logging (recommended)
//   - LowPriority()          - Run on "low" queue
//   - DefaultPriority()      - Run on "default" queue
//   - CriticalPriority()     - Run on "critical" queue
//   - WithMaxRetry(n)        - Set max retry attempts
func GetScheduledTasks() []queue.ScheduledTask {
	return []queue.ScheduledTask{
		// ┌─────────────────────────────────────────────────────────────────┐
		// │                     Server Maintenance                          │
		// └─────────────────────────────────────────────────────────────────┘

		// Clean up old metrics data (older than 7 days) - runs daily at 2 AM
		At("0 2 * * *", serverjobs.NewCleanupOldMetricsTask,
			WithName("cleanup-old-metrics"),
			LowPriority(),
		),

		// ┌─────────────────────────────────────────────────────────────────┐
		// │                     Backup Jobs                                 │
		// └─────────────────────────────────────────────────────────────────┘

		// Example: Prune old backups - runs daily at 3 AM
		// At("0 3 * * *", backupjobs.NewPruneOldBackupsTask,
		//     WithName("prune-old-backups"),
		//     LowPriority(),
		// ),

		// ┌─────────────────────────────────────────────────────────────────┐
		// │                     Load Balancer Health Checks                 │
		// └─────────────────────────────────────────────────────────────────┘

		// Poll all LB backend health endpoints every minute
		At("*/1 * * * *", serverjobs.NewCheckLBBackendHealthTask,
			WithName("check-lb-backend-health"),
		),

		// ┌─────────────────────────────────────────────────────────────────┐
		// │                     Daemon & Queue Status Sync                  │
		// └─────────────────────────────────────────────────────────────────┘

		// Sync daemon status for all connected servers - runs daily at midnight
		Daily(serverjobs.NewSyncAllDaemonsTask,
			WithName("sync-all-daemons"),
			LowPriority(),
		),

		// Sync queue status for all sites on connected servers - runs daily at midnight
		Daily(sitejobs.NewSyncAllQueuesTask,
			WithName("sync-all-queues"),
			LowPriority(),
		),

		// ┌─────────────────────────────────────────────────────────────────┐
		// │                     Server Connectivity Checks                  │
		// └─────────────────────────────────────────────────────────────────┘

		// Check connectivity for servers not updated in the last week - runs daily at midnight
		Daily(serverjobs.NewCheckAllConnectivityTask,
			WithName("check-all-connectivity"),
			LowPriority(),
		),

		// ┌─────────────────────────────────────────────────────────────────┐
		// │                     Site Health Checks                          │
		// └─────────────────────────────────────────────────────────────────┘

		// Example: Check site uptime
		// Every5Minutes(sitejobs.NewCheckSiteHealthTask,
		//     WithName("check-site-health"),
		// ),

		// ┌─────────────────────────────────────────────────────────────────┐
		// │                     Billing & Subscriptions                     │
		// └─────────────────────────────────────────────────────────────────┘

		// Example: Sync subscription status with payment provider
		// Daily(billingjobs.NewSyncSubscriptionsTask,
		//     WithName("sync-subscriptions"),
		// ),

		// ┌─────────────────────────────────────────────────────────────────┐
		// │                     Notifications                               │
		// └─────────────────────────────────────────────────────────────────┘

		// Example: Send daily digest - weekdays at 9 AM
		// At("0 9 * * 1-5", notificationjobs.NewSendDailyDigestTask,
		//     WithName("send-daily-digest"),
		// ),
	}
}

// GetValidScheduledTasks returns only successfully initialized tasks.
// Use this instead of GetScheduledTasks if you want to silently skip failed tasks.
func GetValidScheduledTasks() []queue.ScheduledTask {
	tasks := GetScheduledTasks()
	valid := make([]queue.ScheduledTask, 0, len(tasks))

	for _, t := range tasks {
		if t.Task != nil {
			valid = append(valid, t)
		}
	}

	return valid
}
