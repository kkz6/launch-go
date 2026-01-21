package jobs

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeSyncDaemons = "server:sync_daemons"

type SyncDaemonsPayload struct {
	ServerID string  `json:"server_id"`
	UserID   *string `json:"user_id,omitempty"`
}

// DaemonStatusInfo represents the info stored in the daemon's info field
type DaemonStatusInfo struct {
	Uptime string `json:"uptime"`
	State  string `json:"state"`
	Error  string `json:"error,omitempty"`
	PID    string `json:"pid,omitempty"`
}

// SyncDaemonsJob handles synchronizing daemon status from the server
type SyncDaemonsJob struct {
	pkgjobs.BaseJob[*JobContext, SyncDaemonsPayload]
}

// Handle executes the sync daemons job
func (j *SyncDaemonsJob) Handle(ctx context.Context) error {
	// Get server
	server, err := j.Ctx.Repos().Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	// Get all daemons for this server
	daemons, err := j.Ctx.Repos().Daemon().FindByServer(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to get daemons: %w", err)
	}

	if len(daemons) == 0 {
		j.Ctx.LogInfo("No daemons to sync", "server_id", server.ID)
		return nil
	}

	// Create and run the daemon status check task
	task := tasks.CheckDaemonStatus()
	result, err := j.Ctx.ForServer(server).RunTask(task).AsRoot().Dispatch(ctx)
	if err != nil {
		j.Ctx.LogError(err, "Failed to check daemon status", "server_id", server.ID)
		return err
	}

	// Parse the output using shared parser
	output := result.GetOutput()
	statuses := tasks.ParseDaemonStatusOutput(output)

	// Create a map of daemon ID to status for quick lookup
	// Daemons use program name format: daemon-{id}
	statusMap := make(map[string]*tasks.DaemonStatus)
	for i := range statuses {
		// Extract the actual daemon ID from the program name
		// Format is either "daemon-{id}" or just "{id}"
		daemonID := strings.TrimPrefix(statuses[i].DaemonID, "daemon-")
		statusMap[daemonID] = &statuses[i]
	}

	// Update each daemon with its status
	now := time.Now()
	for i := range daemons {
		daemon := &daemons[i]
		daemon.LastStatusCheck = &now

		// Look for status matching this daemon's ID
		if status, ok := statusMap[daemon.ID]; ok {
			daemon.Running = strings.EqualFold(status.Status, "RUNNING")

			// Build info map
			info := map[string]interface{}{
				"uptime": tasks.FormatUptime(status.UptimeSeconds),
				"state":  status.Status,
				"pid":    status.PID,
			}
			if status.Error != "" {
				info["error"] = status.Error
			}
			daemon.SetInfo(info)
		} else {
			// Daemon not found in supervisor status - mark as not running
			daemon.Running = false
			daemon.SetInfo(map[string]interface{}{
				"state": "NOT_FOUND",
				"error": "Daemon not found in supervisor",
			})
		}

		// Update the daemon in the database
		if err := j.Ctx.Repos().Daemon().Update(ctx, daemon); err != nil {
			j.Ctx.LogError(err, "Failed to update daemon status", "daemon_id", daemon.ID)
		}
	}

	j.Ctx.LogInfo("Daemon status sync completed", "server_id", server.ID, "daemon_count", len(daemons))

	// Broadcast status update
	j.Ctx.BroadcastServerEvent(server, "daemons.synced", map[string]interface{}{
		"server_id":    server.ID,
		"daemon_count": len(daemons),
	})

	return nil
}

// Failed handles job failure
func (j *SyncDaemonsJob) Failed(ctx context.Context, err error) {
	j.Ctx.LogError(err, "Sync daemons job failed", "server_id", j.Payload.ServerID)
}

func NewSyncDaemonsJob(ctx *JobContext, payload SyncDaemonsPayload) *SyncDaemonsJob {
	return &SyncDaemonsJob{
		BaseJob: pkgjobs.NewBaseJob(ctx, payload),
	}
}

func NewSyncDaemonsTask(serverID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.NewTask(TypeSyncDaemons, SyncDaemonsPayload{
		ServerID: serverID,
		UserID:   userID,
	})
}
