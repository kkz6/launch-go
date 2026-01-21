package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/site/tasks"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeSyncQueues = "site:sync_queues"

// SyncQueuesPayload holds data for queue status synchronization
type SyncQueuesPayload struct {
	SiteID   string  `json:"site_id"`
	ServerID string  `json:"server_id"`
	UserID   *string `json:"user_id,omitempty"`
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
	pkgjobs.BaseJob[*JobContext, SyncQueuesPayload]
}

// NewSyncQueuesJob creates a new SyncQueuesJob with the given context and payload
func NewSyncQueuesJob(ctx *JobContext, payload SyncQueuesPayload) *SyncQueuesJob {
	return &SyncQueuesJob{
		BaseJob: pkgjobs.NewBaseJob(ctx, payload),
	}
}

// Handle executes the sync queues job
func (j *SyncQueuesJob) Handle(ctx context.Context) error {
	// Get site
	site, err := j.Ctx.SiteRepo.FindByID(ctx, j.Payload.SiteID)
	if err != nil {
		return fmt.Errorf("failed to find site: %w", err)
	}

	// Get server
	server, err := j.Ctx.ServerRepos.Server().FindByID(ctx, site.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	// Get all queues for this site
	queues, err := j.Ctx.QueueRepo.FindBySite(ctx, j.Payload.SiteID)
	if err != nil {
		return fmt.Errorf("failed to get queues: %w", err)
	}

	if len(queues) == 0 {
		j.Ctx.LogInfo("No queues to sync", "site_id", site.ID)
		return nil
	}

	// Create and run the daemon status check task
	task := tasks.CheckDaemonStatus()
	result, err := j.Ctx.RunTaskOnServer(server, task).AsRoot().Dispatch(ctx)
	if err != nil {
		j.Ctx.LogError(err, "Failed to check daemon status", "site_id", site.ID)
		return err
	}

	// Parse the output using shared parser
	output := result.GetOutput()
	statuses := tasks.ParseDaemonStatusOutput(output)

	// Create a map of daemon ID to status for quick lookup
	statusMap := make(map[string]*tasks.DaemonStatus)
	for i := range statuses {
		statusMap[statuses[i].DaemonID] = &statuses[i]
	}

	// Update each queue with its status
	now := time.Now()
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
		if err := j.Ctx.QueueRepo.Update(ctx, queue); err != nil {
			j.Ctx.LogError(err, "Failed to update queue status", "queue_id", queue.ID)
		}
	}

	j.Ctx.LogInfo("Queue status sync completed", "site_id", site.ID, "queue_count", len(queues))

	// Broadcast status update
	j.Ctx.BroadcastServerEvent(server, "queues.synced", map[string]interface{}{
		"site_id":     site.ID,
		"queue_count": len(queues),
	})

	return nil
}

// Failed handles job failure
func (j *SyncQueuesJob) Failed(ctx context.Context, err error) {
	j.Ctx.LogError(err, "Sync queues job failed", "site_id", j.Payload.SiteID)
}

// NewSyncQueuesTask creates a sync queues status job
func NewSyncQueuesTask(siteID, serverID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.NewTask(TypeSyncQueues, SyncQueuesPayload{
		SiteID:   siteID,
		ServerID: serverID,
		UserID:   userID,
	})
}
