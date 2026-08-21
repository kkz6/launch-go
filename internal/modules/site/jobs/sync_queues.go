package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/notification/notifications"
	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	servertasks "github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	"github.com/kkz6/launch-go/internal/modules/site/tasks"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeSyncQueues = "site:sync_queues"

const syncQueuesJobTimeout = 2 * time.Minute

type SyncQueuesTrigger string

const (
	SyncQueuesTriggerManual      SyncQueuesTrigger = "manual"
	SyncQueuesTriggerScheduled   SyncQueuesTrigger = "scheduled"
	SyncQueuesTriggerPostRestart SyncQueuesTrigger = "post_restart"
)

var errDaemonStatusTimeout = errors.New("daemon status check timed out")

// SyncQueuesPayload holds data for queue status synchronization
type SyncQueuesPayload struct {
	SiteID   string            `json:"site_id"`
	ServerID string            `json:"server_id"`
	UserID   *string           `json:"user_id,omitempty"`
	Trigger  SyncQueuesTrigger `json:"trigger,omitempty"`
}

// DaemonStatusInfo represents the info stored in the queue's info field
type DaemonStatusInfo struct {
	Uptime string `json:"uptime"`
	State  string `json:"state"`
	Error  string `json:"error,omitempty"`
	PID    string `json:"pid,omitempty"`
}

// SyncQueuesJob handles synchronizing queue worker status from the server
type SyncQueuesJob struct {
	Deps    *JobDeps
	Payload SyncQueuesPayload

	// Model fields for Failed() callback
	site           *models.Site
	server         *servermodels.Server
	failedQueueIDs []string
}

func NewSyncQueuesJob(p SyncQueuesPayload) pkgjobs.Handler {
	return &SyncQueuesJob{Deps: deps, Payload: p}
}

// Handle executes the sync queues job
func (j *SyncQueuesJob) Handle(ctx context.Context) error {
	var err error

	// Get site
	j.site, err = j.Deps.Repos.Site().FindByID(ctx, j.Payload.SiteID)
	if err != nil {
		return fmt.Errorf("failed to find site: %w", err)
	}

	// Get server
	j.server, err = j.Deps.ServerRepos.Server().FindByID(ctx, j.site.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	// Get all queues for this site
	queues, err := j.Deps.Repos.Queue().FindBySite(ctx, j.Payload.SiteID)
	if err != nil {
		return fmt.Errorf("failed to get queues: %w", err)
	}

	if len(queues) == 0 {
		j.Deps.Logger.Info().Str("site_id", j.site.ID).Msg("No queues to sync")
		return nil
	}

	// Create and run the daemon status check task
	task := tasks.CheckDaemonStatus()
	result, err := j.Deps.RunTask(j.server, task).AsRoot().Dispatch(ctx)
	if err != nil {
		j.Deps.Logger.Error().Err(err).Str("site_id", j.site.ID).Msg("Failed to check daemon status")
		return fmt.Errorf("check daemon status: %w", err)
	}
	if err := validateDaemonStatusResult(result, task.Timeout()); err != nil {
		if isScheduledSyncTimeout(j.Payload.Trigger, j.Payload.UserID, err) {
			j.Deps.Logger.Warn().Err(err).Str("site_id", j.site.ID).Msg("Scheduled queue sync timed out; keeping the previous status")
			return nil
		}
		j.Deps.Logger.Warn().Err(err).Str("site_id", j.site.ID).Msg("Daemon status check did not complete")
		return err
	}

	// Parse the output using shared parser
	output := result.GetOutput()
	statuses := tasks.ParseDaemonStatusOutput(output)
	j.failedQueueIDs = nil

	// Create a map of daemon ID to status for quick lookup
	statusMap := make(map[string]*tasks.DaemonStatus)
	for i := range statuses {
		statusMap[statuses[i].DaemonID] = &statuses[i]
	}

	// Update each queue with its status
	now := time.Now()
	var updateErrors []error
	for i := range queues {
		queue := &queues[i]
		queue.LastStatusCheck = &now

		// Look for status matching this queue's ID
		if status, ok := statusMap[queue.ID]; ok {
			queue.Running = strings.ToUpper(status.Status) == "RUNNING"

			// Build info JSON
			info := DaemonStatusInfo{
				Uptime: tasks.FormatUptime(status.UptimeSeconds),
				State:  status.Status,
				PID:    status.PID,
			}
			if status.Error != "" {
				info.Error = status.Error
			}

			infoJSON, err := json.Marshal(info)
			if err == nil {
				infoStr := string(infoJSON)
				queue.Info = &infoStr
			}
		} else {
			// Queue not found in supervisor status - mark as not running
			queue.Running = false
			info := DaemonStatusInfo{
				State: "NOT_FOUND",
				Error: "Daemon not found in supervisor",
			}
			infoJSON, err := json.Marshal(info)
			if err == nil {
				infoStr := string(infoJSON)
				queue.Info = &infoStr
			}
		}

		// Update the queue in the database
		if err := j.Deps.Repos.Queue().Update(ctx, queue); err != nil {
			j.Deps.Logger.Error().Err(err).Str("queue_id", queue.ID).Msg("Failed to update queue status")
			updateErrors = append(updateErrors, fmt.Errorf("update queue %s: %w", queue.ID, err))
			continue
		}

		if !queue.Running && queue.InstalledAt != nil && queue.InstallationFailedAt == nil {
			j.failedQueueIDs = append(j.failedQueueIDs, queue.ID)
		}
	}
	if len(updateErrors) > 0 {
		return fmt.Errorf("update queue statuses: %w", errors.Join(updateErrors...))
	}

	j.Deps.Logger.Info().
		Str("site_id", j.site.ID).
		Int("queue_count", len(queues)).
		Msg("Queue status sync completed")

	// Broadcast status update
	j.Deps.BroadcastServerEvent(j.server, "queues.synced", map[string]interface{}{
		"site_id":          j.site.ID,
		"queue_count":      len(queues),
		"failed_count":     len(j.failedQueueIDs),
		"failed_queue_ids": j.failedQueueIDs,
	})

	if len(j.failedQueueIDs) > 0 {
		j.Deps.BroadcastServerEvent(j.server, "queues.failed", map[string]interface{}{
			"site_id":          j.site.ID,
			"queue_count":      len(queues),
			"failed_count":     len(j.failedQueueIDs),
			"failed_queue_ids": j.failedQueueIDs,
		})
		j.notifyStoppedQueues(ctx, queues)
	}

	return nil
}

func (j *SyncQueuesJob) notifyStoppedQueues(ctx context.Context, queues []models.Queue) {
	if j.Deps.TaskRunnerDeps == nil || j.Deps.TaskRunnerDeps.Notifier == nil || j.site == nil || j.server == nil {
		return
	}

	failedIDs := make(map[string]struct{}, len(j.failedQueueIDs))
	for _, id := range j.failedQueueIDs {
		failedIDs[id] = struct{}{}
	}

	queueNames := make([]string, 0, len(failedIDs))
	for i := range queues {
		if _, failed := failedIDs[queues[i].ID]; failed {
			queueNames = append(queueNames, queues[i].QueueName)
		}
	}
	if len(queueNames) == 0 {
		return
	}

	notification := notifications.NewQueueStoppedNotification(j.site.Address, j.server.Name, queueNames).
		WithQueuesURL(queueManagementURL(j.Deps.FrontendURL, j.server.ID, j.site.ID))
	if err := j.Deps.TaskRunnerDeps.Notifier.SendToTeam(ctx, j.server.TeamID, notification); err != nil {
		j.Deps.Logger.Error().Err(err).Str("site_id", j.site.ID).Msg("failed to send stopped queue notification")
	}
}

func queueManagementURL(frontendURL, serverID, siteID string) string {
	if frontendURL == "" {
		return ""
	}
	return fmt.Sprintf("%s/servers/%s/sites/%s?tab=queues", strings.TrimRight(frontendURL, "/"), serverID, siteID)
}

// Failed handles job failure
func (j *SyncQueuesJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).Str("site_id", j.Payload.SiteID).Msg("Sync queues job failed")
}

// NewSyncQueuesTask creates a sync queues status job
func NewSyncQueuesTask(siteID, serverID string, userID *string) (*asynq.Task, error) {
	trigger := SyncQueuesTriggerManual
	if userID == nil {
		trigger = SyncQueuesTriggerScheduled
	}

	return pkgjobs.Task(TypeSyncQueues, SyncQueuesPayload{
		SiteID:   siteID,
		ServerID: serverID,
		UserID:   userID,
		Trigger:  trigger,
	}, asynq.Timeout(syncQueuesJobTimeout))
}

func validateDaemonStatusResult(result *servertasks.TaskRunnerResult, timeout time.Duration) error {
	if result == nil {
		return errors.New("daemon status check returned no result")
	}
	if result.Error != nil {
		return fmt.Errorf("daemon status check failed: %w", result.Error)
	}
	if result.TaskResult != nil && result.TaskResult.IsTimedOut() {
		return fmt.Errorf("%w after %s", errDaemonStatusTimeout, timeout)
	}
	if !result.IsSuccessful() {
		return fmt.Errorf("daemon status check failed with exit code %d", result.GetExitCode())
	}
	return nil
}

func isScheduledSyncTimeout(trigger SyncQueuesTrigger, userID *string, err error) bool {
	isScheduled := trigger == SyncQueuesTriggerScheduled || (trigger == "" && userID == nil)
	return isScheduled && (errors.Is(err, context.DeadlineExceeded) || errors.Is(err, errDaemonStatusTimeout))
}
