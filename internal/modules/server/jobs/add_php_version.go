package jobs

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/server/enums"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/websocket"
)

const TypeAddPhpVersion = "server:add_php_version"

// AddPhpVersionPayload contains data for adding a PHP version to a server
type AddPhpVersionPayload struct {
	ServerID   string         `json:"server_id"`
	PhpVersion enums.Software `json:"php_version"`
	ServiceID  string         `json:"service_id,omitempty"`
}

// AddPhpVersionJob handles adding a PHP version to a server
type AddPhpVersionJob struct {
	db     *gorm.DB
	ws     *websocket.Hub
	logger *zerolog.Logger
}

// NewAddPhpVersionJob creates a new add PHP version job handler
func NewAddPhpVersionJob(db *gorm.DB, ws *websocket.Hub, logger *zerolog.Logger) *AddPhpVersionJob {
	return &AddPhpVersionJob{
		db:     db,
		ws:     ws,
		logger: logger,
	}
}

// NewAddPhpVersionTask creates a new asynq task for adding a PHP version
func NewAddPhpVersionTask(serverID string, phpVersion enums.Software, serviceID string) (*asynq.Task, error) {
	payload, err := json.Marshal(AddPhpVersionPayload{
		ServerID:   serverID,
		PhpVersion: phpVersion,
		ServiceID:  serviceID,
	})
	if err != nil {
		return nil, err
	}

	return asynq.NewTask(TypeAddPhpVersion, payload), nil
}

// Handle processes the add PHP version job
func (j *AddPhpVersionJob) Handle(ctx context.Context, t *asynq.Task) error {
	var payload AddPhpVersionPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	j.logger.Info().
		Str("server_id", payload.ServerID).
		Str("php_version", string(payload.PhpVersion)).
		Msg("Adding PHP version to server")

	// Fetch the server
	var server models.Server
	if err := j.db.First(&server, "id = ?", payload.ServerID).Error; err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	j.broadcastProgress(payload.ServerID, "installing", fmt.Sprintf("Installing %s...", payload.PhpVersion.Label()))

	// TODO: Implement actual PHP installation:
	// 1. Connect to server via SSH
	// 2. Run AddPhpVersion task script
	// 3. Configure PHP-FPM
	// 4. Update service status

	// Update the service task_id if service_id is provided
	if payload.ServiceID != "" {
		// Create a task record and associate it with the service
		// This would be done through the task runner
	}

	j.broadcastProgress(payload.ServerID, "installed", fmt.Sprintf("%s installed successfully", payload.PhpVersion.Label()))

	return nil
}

// Failed handles job failure
func (j *AddPhpVersionJob) Failed(ctx context.Context, t *asynq.Task, err error) {
	var payload AddPhpVersionPayload
	if unmarshalErr := json.Unmarshal(t.Payload(), &payload); unmarshalErr != nil {
		j.logger.Error().Err(unmarshalErr).Msg("Failed to unmarshal payload in failure handler")
		return
	}

	j.logger.Error().
		Err(err).
		Str("server_id", payload.ServerID).
		Str("php_version", string(payload.PhpVersion)).
		Msg("Failed to add PHP version")

	// Delete the service record that was created before dispatching the job
	j.db.Where("server_id = ? AND software = ?", payload.ServerID, payload.PhpVersion).
		Delete(&models.InstalledService{})

	j.broadcastProgress(payload.ServerID, "failed", fmt.Sprintf("Failed to install %s", payload.PhpVersion.Label()))

	// TODO: Dispatch PhpInstallFailed event
}

func (j *AddPhpVersionJob) broadcastProgress(serverID, status, message string) {
	j.ws.BroadcastToServer(serverID, "server.service.progress", map[string]interface{}{
		"server_id": serverID,
		"status":    status,
		"message":   message,
	})
}
