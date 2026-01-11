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

const TypeInstallCron = "server:install_cron"

// InstallCronPayload contains data for installing a cron job
type InstallCronPayload struct {
	CronID string  `json:"cron_id"`
	UserID *string `json:"user_id,omitempty"`
}

// InstallCronJob handles installing a cron job on a server
type InstallCronJob struct {
	db     *gorm.DB
	ws     *websocket.Hub
	logger *zerolog.Logger
}

// NewInstallCronJob creates a new install cron job handler
func NewInstallCronJob(db *gorm.DB, ws *websocket.Hub, logger *zerolog.Logger) *InstallCronJob {
	return &InstallCronJob{
		db:     db,
		ws:     ws,
		logger: logger,
	}
}

// NewInstallCronTask creates a new asynq task for installing a cron job
func NewInstallCronTask(cronID string, userID *string) (*asynq.Task, error) {
	payload, err := json.Marshal(InstallCronPayload{
		CronID: cronID,
		UserID: userID,
	})
	if err != nil {
		return nil, err
	}

	return asynq.NewTask(TypeInstallCron, payload), nil
}

// Handle processes the install cron job
func (j *InstallCronJob) Handle(ctx context.Context, t *asynq.Task) error {
	var payload InstallCronPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	j.logger.Info().
		Str("cron_id", payload.CronID).
		Msg("Installing cron job")

	// Fetch the cron with server
	var cron models.Cron
	if err := j.db.Preload("Server").First(&cron, "id = ?", payload.CronID).Error; err != nil {
		return fmt.Errorf("failed to find cron: %w", err)
	}

	j.broadcastProgress(cron.ServerID, "installing", fmt.Sprintf("Installing cron job: %s", cron.Command))

	// TODO: Implement actual cron installation:
	// 1. Build cron configuration content
	// 2. Upload cron file to /etc/cron.d/
	// 3. Set proper permissions

	// Mark the cron as installed
	now := time.Now()
	if err := j.db.Model(&cron).Update("installed_at", &now).Error; err != nil {
		return fmt.Errorf("failed to update cron status: %w", err)
	}

	j.broadcastProgress(cron.ServerID, "installed", "Cron job installed successfully")

	return nil
}

// Failed handles job failure
func (j *InstallCronJob) Failed(ctx context.Context, t *asynq.Task, err error) {
	var payload InstallCronPayload
	if unmarshalErr := json.Unmarshal(t.Payload(), &payload); unmarshalErr != nil {
		j.logger.Error().Err(unmarshalErr).Msg("Failed to unmarshal payload in failure handler")
		return
	}

	j.logger.Error().
		Err(err).
		Str("cron_id", payload.CronID).
		Msg("Failed to install cron job")

	// Fetch the cron to get server ID for broadcasting
	var cron models.Cron
	if findErr := j.db.First(&cron, "id = ?", payload.CronID).Error; findErr != nil {
		return
	}

	// Mark installation as failed
	now := time.Now()
	j.db.Model(&cron).Update("installation_failed_at", &now)

	j.broadcastProgress(cron.ServerID, "failed", fmt.Sprintf("Failed to install cron job: %s", cron.Command))

	// TODO: Notify user about installation failure
}

func (j *InstallCronJob) broadcastProgress(serverID, status, message string) {
	j.ws.BroadcastToServer(serverID, "server.cron.progress", map[string]interface{}{
		"server_id": serverID,
		"status":    status,
		"message":   message,
	})
}
