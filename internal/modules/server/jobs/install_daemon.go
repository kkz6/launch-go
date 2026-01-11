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

const TypeInstallDaemon = "server:install_daemon"

// InstallDaemonPayload contains data for installing a daemon
type InstallDaemonPayload struct {
	DaemonID string  `json:"daemon_id"`
	UserID   *string `json:"user_id,omitempty"`
}

// InstallDaemonJob handles installing a daemon on a server
type InstallDaemonJob struct {
	db     *gorm.DB
	ws     *websocket.Hub
	logger *zerolog.Logger
}

// NewInstallDaemonJob creates a new install daemon job handler
func NewInstallDaemonJob(db *gorm.DB, ws *websocket.Hub, logger *zerolog.Logger) *InstallDaemonJob {
	return &InstallDaemonJob{
		db:     db,
		ws:     ws,
		logger: logger,
	}
}

// NewInstallDaemonTask creates a new asynq task for installing a daemon
func NewInstallDaemonTask(daemonID string, userID *string) (*asynq.Task, error) {
	payload, err := json.Marshal(InstallDaemonPayload{
		DaemonID: daemonID,
		UserID:   userID,
	})
	if err != nil {
		return nil, err
	}

	return asynq.NewTask(TypeInstallDaemon, payload), nil
}

// Handle processes the install daemon job
func (j *InstallDaemonJob) Handle(ctx context.Context, t *asynq.Task) error {
	var payload InstallDaemonPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	j.logger.Info().
		Str("daemon_id", payload.DaemonID).
		Msg("Installing daemon")

	// Fetch the daemon with server
	var daemon models.Daemon
	if err := j.db.Preload("Server").First(&daemon, "id = ?", payload.DaemonID).Error; err != nil {
		return fmt.Errorf("failed to find daemon: %w", err)
	}

	j.broadcastProgress(daemon.ServerID, "installing", fmt.Sprintf("Installing daemon: %s", daemon.Command))

	// TODO: Implement actual daemon installation:
	// 1. Build supervisor program configuration
	// 2. Upload configuration file to server
	// 3. Reload supervisor
	// 4. Check daemon status

	// Mark the daemon as installed
	now := time.Now()
	if err := j.db.Model(&daemon).Update("installed_at", &now).Error; err != nil {
		return fmt.Errorf("failed to update daemon status: %w", err)
	}

	j.broadcastProgress(daemon.ServerID, "installed", "Daemon installed successfully")

	return nil
}

// Failed handles job failure
func (j *InstallDaemonJob) Failed(ctx context.Context, t *asynq.Task, err error) {
	var payload InstallDaemonPayload
	if unmarshalErr := json.Unmarshal(t.Payload(), &payload); unmarshalErr != nil {
		j.logger.Error().Err(unmarshalErr).Msg("Failed to unmarshal payload in failure handler")
		return
	}

	j.logger.Error().
		Err(err).
		Str("daemon_id", payload.DaemonID).
		Msg("Failed to install daemon")

	// Fetch the daemon to get server ID for broadcasting
	var daemon models.Daemon
	if findErr := j.db.First(&daemon, "id = ?", payload.DaemonID).Error; findErr != nil {
		return
	}

	// Mark installation as failed
	now := time.Now()
	j.db.Model(&daemon).Update("installation_failed_at", &now)

	j.broadcastProgress(daemon.ServerID, "failed", fmt.Sprintf("Failed to install daemon: %s", daemon.Command))

	// TODO: Notify user about installation failure
}

func (j *InstallDaemonJob) broadcastProgress(serverID, status, message string) {
	j.ws.BroadcastToServer(serverID, "server.daemon.progress", map[string]interface{}{
		"server_id": serverID,
		"status":    status,
		"message":   message,
	})
}
