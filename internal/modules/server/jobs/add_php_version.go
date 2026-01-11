package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/enums"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/pkg/jobs"
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
	*JobContext
	jobs.InstallationTracker
}

// NewAddPhpVersionTask creates a new asynq task for adding a PHP version
func NewAddPhpVersionTask(serverID string, phpVersion enums.Software, serviceID string) (*asynq.Task, error) {
	return jobs.NewTask(TypeAddPhpVersion, AddPhpVersionPayload{
		ServerID:   serverID,
		PhpVersion: phpVersion,
		ServiceID:  serviceID,
	})
}

// Handle processes the add PHP version job
func (j *AddPhpVersionJob) Handle(ctx context.Context, t *asynq.Task) error {
	payload, err := jobs.UnmarshalPayload[AddPhpVersionPayload](t)
	if err != nil {
		return err
	}

	j.Logger.Info().
		Str("server_id", payload.ServerID).
		Str("php_version", string(payload.PhpVersion)).
		Msg("Adding PHP version to server")

	// Fetch the server
	server, err := j.FindServer(ctx, payload.ServerID)
	if err != nil {
		return err
	}

	j.broadcastProgress(payload.ServerID, "installing", fmt.Sprintf("Installing %s...", payload.PhpVersion.Label()))

	// Run the PHP installation task with tracking
	result, err := j.RunTask(server, tasks.NewAddPhpVersion(server, payload.PhpVersion)).
		AsRoot().
		KeepTrack().
		Dispatch(ctx)
	if err != nil {
		return fmt.Errorf("failed to install PHP version: %w", err)
	}

	// Update the service task_id to link to the tracked task
	if result.TaskModel != nil {
		j.DB.Model(&models.InstalledService{}).
			Where("server_id = ? AND software = ?", payload.ServerID, payload.PhpVersion).
			Update("task_id", result.TaskModel.ID)
	}

	j.broadcastProgress(payload.ServerID, "installed", fmt.Sprintf("%s installed successfully", payload.PhpVersion.Label()))

	return nil
}

// Failed handles job failure
func (j *AddPhpVersionJob) Failed(ctx context.Context, t *asynq.Task, err error) {
	payload, unmarshalErr := jobs.UnmarshalPayload[AddPhpVersionPayload](t)
	if unmarshalErr != nil {
		j.Logger.Error().Err(unmarshalErr).Msg("Failed to unmarshal payload in failure handler")
		return
	}

	j.Logger.Error().
		Err(err).
		Str("server_id", payload.ServerID).
		Str("php_version", string(payload.PhpVersion)).
		Msg("Failed to add PHP version")

	// Delete the service record that was created before dispatching the job
	j.DB.Where("server_id = ? AND software = ?", payload.ServerID, payload.PhpVersion).
		Delete(&models.InstalledService{})

	j.broadcastProgress(payload.ServerID, "failed", fmt.Sprintf("Failed to install %s", payload.PhpVersion.Label()))
}

func (j *AddPhpVersionJob) broadcastProgress(serverID, status, message string) {
	j.BroadcastToServer(serverID, "server.service.progress", map[string]interface{}{
		"server_id": serverID,
		"status":    status,
		"message":   message,
	})
}
