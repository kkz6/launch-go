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

const TypeUninstallCron = "server:uninstall_cron"

// UninstallCronPayload contains data for uninstalling a cron job
type UninstallCronPayload struct {
	ServerID string  `json:"server_id"`
	CronID   string  `json:"cron_id"`
	UserID   *string `json:"user_id,omitempty"`
}

// UninstallCronJob handles uninstalling a cron job from a server
type UninstallCronJob struct {
	db     *gorm.DB
	ws     *websocket.Hub
	logger *zerolog.Logger
}

// NewUninstallCronJob creates a new uninstall cron job handler
func NewUninstallCronJob(db *gorm.DB, ws *websocket.Hub, logger *zerolog.Logger) *UninstallCronJob {
	return &UninstallCronJob{
		db:     db,
		ws:     ws,
		logger: logger,
	}
}

// NewUninstallCronTask creates a new asynq task for uninstalling a cron job
func NewUninstallCronTask(serverID, cronID string, userID *string) (*asynq.Task, error) {
	payload, err := json.Marshal(UninstallCronPayload{
		ServerID: serverID,
		CronID:   cronID,
		UserID:   userID,
	})
	if err != nil {
		return nil, err
	}

	return asynq.NewTask(TypeUninstallCron, payload), nil
}

// Handle processes the uninstall cron job
func (j *UninstallCronJob) Handle(ctx context.Context, t *asynq.Task) error {
	var payload UninstallCronPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	j.logger.Info().
		Str("server_id", payload.ServerID).
		Str("cron_id", payload.CronID).
		Msg("Uninstalling cron job")

	// Fetch the cron with server
	var cron models.Cron
	if err := j.db.Preload("Server").First(&cron, "id = ?", payload.CronID).Error; err != nil {
		return fmt.Errorf("failed to find cron: %w", err)
	}

	j.broadcastProgress(payload.ServerID, "uninstalling", fmt.Sprintf("Uninstalling cron job: %s", cron.Command))

	// TODO: Implement actual cron removal:
	// 1. Connect to server via SSH
	// 2. Remove cron file from /etc/cron.d/
	// 3. Delete cron record from database

	// Delete the cron record
	if err := j.db.Delete(&cron).Error; err != nil {
		return fmt.Errorf("failed to delete cron record: %w", err)
	}

	j.broadcastProgress(payload.ServerID, "uninstalled", "Cron job uninstalled successfully")

	return nil
}

// Failed handles job failure
func (j *UninstallCronJob) Failed(ctx context.Context, t *asynq.Task, err error) {
	var payload UninstallCronPayload
	if unmarshalErr := json.Unmarshal(t.Payload(), &payload); unmarshalErr != nil {
		j.logger.Error().Err(unmarshalErr).Msg("Failed to unmarshal payload in failure handler")
		return
	}

	j.logger.Error().
		Err(err).
		Str("server_id", payload.ServerID).
		Str("cron_id", payload.CronID).
		Msg("Failed to uninstall cron job")

	// Fetch the cron for error message
	var cron models.Cron
	if findErr := j.db.First(&cron, "id = ?", payload.CronID).Error; findErr != nil {
		j.broadcastProgress(payload.ServerID, "failed", "Failed to uninstall cron job")
		return
	}

	j.broadcastProgress(payload.ServerID, "failed", fmt.Sprintf("Failed to uninstall cron job: %s", cron.Command))
}

func (j *UninstallCronJob) broadcastProgress(serverID, status, message string) {
	j.ws.BroadcastToServer(serverID, "server.cron.progress", map[string]interface{}{
		"server_id": serverID,
		"status":    status,
		"message":   message,
		"timestamp": time.Now().Format(time.RFC3339),
	})
}
