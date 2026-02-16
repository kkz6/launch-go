package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/hibiken/asynq"

	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/site/models"
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
	Deps    *JobDeps
	Payload SyncQueuesPayload

	// Model fields for Failed() callback
	site   *models.Site
	server *servermodels.Server
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
		if err := j.Deps.Repos.Queue().Update(ctx, queue); err != nil {
			j.Deps.Logger.Error().Err(err).Str("queue_id", queue.ID).Msg("Failed to update queue status")
		}
	}

	j.Deps.Logger.Info().
		Str("site_id", j.site.ID).
		Int("queue_count", len(queues)).
		Msg("Queue status sync completed")

	// Broadcast status update
	j.Deps.BroadcastServerEvent(j.server, "queues.synced", map[string]interface{}{
		"site_id":     j.site.ID,
		"queue_count": len(queues),
	})

	return nil
}

// Failed handles job failure
func (j *SyncQueuesJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).Str("site_id", j.Payload.SiteID).Msg("Sync queues job failed")
}

// NewSyncQueuesTask creates a sync queues status job
func NewSyncQueuesTask(siteID, serverID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.Task(TypeSyncQueues, SyncQueuesPayload{
		SiteID:   siteID,
		ServerID: serverID,
		UserID:   userID,
	})
}
