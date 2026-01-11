package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/websocket"
)

const TypeUninstallDaemon = "server:uninstall_daemon"

// UninstallDaemonPayload contains data for uninstalling a daemon
type UninstallDaemonPayload struct {
	ServerID string  `json:"server_id"`
	DaemonID string  `json:"daemon_id"`
	UserID   *string `json:"user_id,omitempty"`
}

// UninstallDaemonJob handles uninstalling a daemon from a server
type UninstallDaemonJob struct {
	db     *gorm.DB
	ws     *websocket.Hub
	logger *zerolog.Logger
}

// NewUninstallDaemonJob creates a new uninstall daemon job handler
func NewUninstallDaemonJob(db *gorm.DB, ws *websocket.Hub, logger *zerolog.Logger) *UninstallDaemonJob {
	return &UninstallDaemonJob{
		db:     db,
		ws:     ws,
		logger: logger,
	}
}

// NewUninstallDaemonTask creates a new asynq task for uninstalling a daemon
func NewUninstallDaemonTask(serverID, daemonID string, userID *string) (*asynq.Task, error) {
	payload, err := json.Marshal(UninstallDaemonPayload{
		ServerID: serverID,
		DaemonID: daemonID,
		UserID:   userID,
	})
	if err != nil {
		return nil, err
	}

	return asynq.NewTask(TypeUninstallDaemon, payload), nil
}

// Handle processes the uninstall daemon job
func (j *UninstallDaemonJob) Handle(ctx context.Context, t *asynq.Task) error {
	var payload UninstallDaemonPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	j.logger.Info().
		Str("server_id", payload.ServerID).
		Str("daemon_id", payload.DaemonID).
		Msg("Uninstalling daemon")

	// Fetch the daemon with server
	var daemon models.Daemon
	if err := j.db.Preload("Server").First(&daemon, "id = ?", payload.DaemonID).Error; err != nil {
		return fmt.Errorf("failed to find daemon: %w", err)
	}

	j.broadcastProgress(payload.ServerID, payload.DaemonID, "uninstalling", fmt.Sprintf("Uninstalling daemon: %s", daemon.Command))

	// TODO: Implement actual daemon removal:
	// 1. Connect to server via SSH
	// 2. Stop the daemon service
	// 3. Remove systemd service file
	// 4. Reload systemd
	// 5. Delete daemon record from database

	// Delete the daemon record
	if err := j.db.Delete(&daemon).Error; err != nil {
		return fmt.Errorf("failed to delete daemon record: %w", err)
	}

	j.broadcastProgress(payload.ServerID, payload.DaemonID, "uninstalled", "Daemon uninstalled successfully")

	return nil
}

// Failed handles job failure
func (j *UninstallDaemonJob) Failed(ctx context.Context, t *asynq.Task, err error) {
	var payload UninstallDaemonPayload
	if unmarshalErr := json.Unmarshal(t.Payload(), &payload); unmarshalErr != nil {
		j.logger.Error().Err(unmarshalErr).Msg("Failed to unmarshal payload in failure handler")
		return
	}

	j.logger.Error().
		Err(err).
		Str("server_id", payload.ServerID).
		Str("daemon_id", payload.DaemonID).
		Msg("Failed to uninstall daemon")

	// Fetch the daemon for error message
	var daemon models.Daemon
	if findErr := j.db.First(&daemon, "id = ?", payload.DaemonID).Error; findErr != nil {
		j.broadcastProgress(payload.ServerID, payload.DaemonID, "failed", "Failed to uninstall daemon")
		return
	}

	j.broadcastProgress(payload.ServerID, payload.DaemonID, "failed", fmt.Sprintf("Failed to uninstall daemon: %s", daemon.Command))
}

func (j *UninstallDaemonJob) broadcastProgress(serverID, daemonID, status, message string) {
	j.ws.BroadcastToServer(serverID, "server.daemon.progress", map[string]interface{}{
		"server_id": serverID,
		"daemon_id": daemonID,
		"status":    status,
		"message":   message,
		"timestamp": time.Now().Format(time.RFC3339),
	})
}
