package jobs

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/models"
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
	Deps    *JobDeps
	Payload SyncDaemonsPayload

	server *models.Server
}

func NewSyncDaemonsJob(p SyncDaemonsPayload) pkgjobs.Handler {
	return &SyncDaemonsJob{Deps: deps, Payload: p}
}

func (j *SyncDaemonsJob) Handle(ctx context.Context) error {
	var err error

	j.server, err = j.Deps.Repos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("find server: %w", err)
	}

	daemons, err := j.Deps.Repos.Daemon().FindByServer(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("get daemons: %w", err)
	}

	if len(daemons) == 0 {
		j.Deps.Logger.Info().
			Str("server_id", j.server.ID).
			Msg("no daemons to sync")
		return nil
	}

	task := tasks.CheckDaemonStatus()
	result, err := j.Deps.RunTask(j.server, task).AsUser().Dispatch(ctx)
	if err != nil {
		j.Deps.Logger.Error().Err(err).
			Str("server_id", j.server.ID).
			Msg("failed to check daemon status")
		return err
	}

	output := result.GetOutput()
	statuses := tasks.ParseDaemonStatusOutput(output)

	statusMap := make(map[string]*tasks.DaemonStatus)
	for i := range statuses {
		daemonID := strings.TrimPrefix(statuses[i].DaemonID, "daemon-")
		statusMap[daemonID] = &statuses[i]
	}

	now := time.Now()
	for i := range daemons {
		daemon := &daemons[i]
		daemon.LastStatusCheck = &now

		if status, ok := statusMap[daemon.ID]; ok {
			daemon.Running = strings.EqualFold(status.Status, "RUNNING")

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
			daemon.Running = false
			daemon.SetInfo(map[string]interface{}{
				"state": "NOT_FOUND",
				"error": "Daemon not found in supervisor",
			})
		}

		if err := j.Deps.Repos.Daemon().Update(ctx, daemon); err != nil {
			j.Deps.Logger.Error().Err(err).
				Str("daemon_id", daemon.ID).
				Msg("failed to update daemon status")
		}
	}

	j.Deps.Logger.Info().
		Str("server_id", j.server.ID).
		Int("daemon_count", len(daemons)).
		Msg("daemon status sync completed")

	j.Deps.BroadcastServerEvent(j.server, "daemons.synced", map[string]interface{}{
		"server_id":    j.server.ID,
		"daemon_count": len(daemons),
	})

	return nil
}

func (j *SyncDaemonsJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).
		Str("server_id", j.Payload.ServerID).
		Msg("sync daemons job failed")
}

func NewSyncDaemonsTask(serverID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.Task(TypeSyncDaemons, SyncDaemonsPayload{
		ServerID: serverID,
		UserID:   userID,
	})
}
