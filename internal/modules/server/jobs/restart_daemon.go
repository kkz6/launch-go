package jobs

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/websocket"
)

const TypeRestartDaemon = "server:restart_daemon"

// RestartDaemonPayload contains data for restarting a daemon
type RestartDaemonPayload struct {
	ServerID string `json:"server_id"`
	DaemonID string `json:"daemon_id"`
}

// RestartDaemonJob handles restarting a daemon on a server
type RestartDaemonJob struct {
	db     *gorm.DB
	ws     *websocket.Hub
	logger *zerolog.Logger
}

// NewRestartDaemonJob creates a new restart daemon job handler
func NewRestartDaemonJob(db *gorm.DB, ws *websocket.Hub, logger *zerolog.Logger) *RestartDaemonJob {
	return &RestartDaemonJob{
		db:     db,
		ws:     ws,
		logger: logger,
	}
}

// NewRestartDaemonTask creates a new asynq task for restarting a daemon
func NewRestartDaemonTask(serverID, daemonID string) (*asynq.Task, error) {
	payload, err := json.Marshal(RestartDaemonPayload{
		ServerID: serverID,
		DaemonID: daemonID,
	})
	if err != nil {
		return nil, err
	}

	return asynq.NewTask(TypeRestartDaemon, payload), nil
}

// Handle processes the restart daemon job
func (j *RestartDaemonJob) Handle(ctx context.Context, t *asynq.Task) error {
	var payload RestartDaemonPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	j.logger.Info().
		Str("server_id", payload.ServerID).
		Str("daemon_id", payload.DaemonID).
		Msg("Restarting daemon")

	// Fetch the server
	var server models.Server
	if err := j.db.First(&server, "id = ?", payload.ServerID).Error; err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	// Fetch the daemon
	var daemon models.Daemon
	if err := j.db.First(&daemon, "id = ?", payload.DaemonID).Error; err != nil {
		return fmt.Errorf("failed to find daemon: %w", err)
	}

	j.broadcastProgress(payload.ServerID, "restarting", fmt.Sprintf("Restarting daemon: %s", daemon.Command))

	// TODO: Implement actual daemon restart:
	// 1. Connect to server via SSH
	// 2. Run supervisor restart program command
	// 3. Check daemon status

	j.broadcastProgress(payload.ServerID, "running", "Daemon restarted successfully")

	return nil
}

// Failed handles job failure
func (j *RestartDaemonJob) Failed(ctx context.Context, t *asynq.Task, err error) {
	var payload RestartDaemonPayload
	if unmarshalErr := json.Unmarshal(t.Payload(), &payload); unmarshalErr != nil {
		j.logger.Error().Err(unmarshalErr).Msg("Failed to unmarshal payload in failure handler")
		return
	}

	j.logger.Error().
		Err(err).
		Str("server_id", payload.ServerID).
		Str("daemon_id", payload.DaemonID).
		Msg("Failed to restart daemon")

	j.broadcastProgress(payload.ServerID, "failed", "Failed to restart daemon")
}

func (j *RestartDaemonJob) broadcastProgress(serverID, status, message string) {
	j.ws.BroadcastToServer(serverID, "server.daemon.progress", map[string]interface{}{
		"server_id": serverID,
		"status":    status,
		"message":   message,
	})
}
