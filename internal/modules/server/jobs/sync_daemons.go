package jobs

import (
	"bufio"
	"context"
	"encoding/json"
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

// daemonStatusResult represents a single daemon status from the server
type daemonStatusResult struct {
	DaemonID      string `json:"daemon_id"`
	Status        string `json:"status"`
	PID           string `json:"pid"`
	UptimeSeconds int    `json:"uptime_seconds"`
	Description   string `json:"description"`
	Error         string `json:"error"`
}

// SyncDaemonsJob handles synchronizing daemon status from the server
type SyncDaemonsJob struct {
	ctx     *JobContext
	Payload SyncDaemonsPayload
}

// Handle executes the sync daemons job
func (j *SyncDaemonsJob) Handle(ctx context.Context) error {
	// Get server
	server, err := j.ctx.Repo.FindServerByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	// Get all daemons for this server
	daemons, err := j.ctx.Repo.FindDaemonsByServer(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to get daemons: %w", err)
	}

	if len(daemons) == 0 {
		j.ctx.LogInfo("No daemons to sync", "server_id", server.ID)
		return nil
	}

	// Create and run the daemon status check task
	task := tasks.CheckDaemonStatus()
	result, err := j.ctx.ForServer(server).RunTask(task).AsRoot().Dispatch(ctx)
	if err != nil {
		j.ctx.LogError(err, "Failed to check daemon status", "server_id", server.ID)
		return err
	}

	// Parse the output
	output := result.GetOutput()
	statuses := j.parseDaemonStatus(output)

	// Create a map of daemon ID to status for quick lookup
	// Daemons use program name format: daemon-{id}
	statusMap := make(map[string]*daemonStatusResult)
	for i := range statuses {
		// Extract the actual daemon ID from the program name
		// Format is either "daemon-{id}" or just "{id}"
		daemonID := statuses[i].DaemonID
		if strings.HasPrefix(daemonID, "daemon-") {
			daemonID = strings.TrimPrefix(daemonID, "daemon-")
		}
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
				"uptime": formatUptime(status.UptimeSeconds),
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
		if err := j.ctx.Repo.UpdateDaemon(ctx, daemon); err != nil {
			j.ctx.LogError(err, "Failed to update daemon status", "daemon_id", daemon.ID)
		}
	}

	j.ctx.LogInfo("Daemon status sync completed", "server_id", server.ID, "daemon_count", len(daemons))

	// Broadcast status update
	j.ctx.BroadcastToServer(server.ID, "daemons.synced", map[string]interface{}{
		"server_id":    server.ID,
		"daemon_count": len(daemons),
	})

	return nil
}

// Failed handles job failure
func (j *SyncDaemonsJob) Failed(ctx context.Context, err error) {
	j.ctx.LogError(err, "Sync daemons job failed", "server_id", j.Payload.ServerID)
}

// parseDaemonStatus parses the output of the daemon status check task
func (j *SyncDaemonsJob) parseDaemonStatus(output string) []daemonStatusResult {
	var results []daemonStatusResult

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and the completion marker
		if line == "" || line == "===STATUS_CHECK_COMPLETE===" {
			continue
		}

		// Try to parse as JSON
		if strings.HasPrefix(line, "{") {
			var status daemonStatusResult
			if err := json.Unmarshal([]byte(line), &status); err == nil {
				results = append(results, status)
			}
		}
	}

	return results
}

func NewSyncDaemonsJob(ctx *JobContext, payload SyncDaemonsPayload) *SyncDaemonsJob {
	return &SyncDaemonsJob{
		ctx:     ctx,
		Payload: payload,
	}
}

func NewSyncDaemonsTask(serverID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.NewTask(TypeSyncDaemons, SyncDaemonsPayload{
		ServerID: serverID,
		UserID:   userID,
	})
}

// formatUptime converts seconds to a human-readable uptime string
func formatUptime(seconds int) string {
	if seconds <= 0 {
		return ""
	}

	days := seconds / 86400
	hours := (seconds % 86400) / 3600
	minutes := (seconds % 3600) / 60
	secs := seconds % 60

	var parts []string
	if days > 0 {
		if days == 1 {
			parts = append(parts, "1 day")
		} else {
			parts = append(parts, fmt.Sprintf("%d days", days))
		}
	}
	if hours > 0 {
		if hours == 1 {
			parts = append(parts, "1 hour")
		} else {
			parts = append(parts, fmt.Sprintf("%d hours", hours))
		}
	}
	if minutes > 0 {
		if minutes == 1 {
			parts = append(parts, "1 minute")
		} else {
			parts = append(parts, fmt.Sprintf("%d minutes", minutes))
		}
	}
	if secs > 0 && len(parts) < 2 {
		if secs == 1 {
			parts = append(parts, "1 second")
		} else {
			parts = append(parts, fmt.Sprintf("%d seconds", secs))
		}
	}

	if len(parts) == 0 {
		return "0 seconds"
	}

	return strings.Join(parts, ", ")
}
