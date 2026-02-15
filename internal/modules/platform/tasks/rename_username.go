package tasks

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"text/template"
	"time"

	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	sitemodels "github.com/kkz6/launch-go/internal/modules/site/models"

	"github.com/kkz6/launch-go/internal/modules/platform/models"
	"github.com/kkz6/launch-go/internal/modules/platform/tasks/templates"
	"github.com/kkz6/launch-go/internal/modules/platform/types"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

const RenameUsernameTaskType = "platform:rename_username"

const RenameUsernameKey = "rename_username_captain"

// renameCallbackData holds data needed for callback handling
type renameCallbackData struct {
	ServerID               string `json:"server_id"`
	TeamID                 string `json:"team_id"`
	ServerPlatformUpdateID string `json:"server_platform_update_id"`
	OldUsername            string `json:"old_username"`
	NewUsername            string `json:"new_username"`
}

// renameUsernameTask implements Task and CallbackPayload interfaces
type renameUsernameTask struct {
	*taskrunner.BaseTask
	callback renameCallbackData
}

// RenameUsernameTask creates a new rename username task
func RenameUsernameTask(server *servermodels.Server, _ *models.PlatformUpdate) taskrunner.Task {
	oldUsername := server.GetUsername()
	newUsername := "captain"

	script := buildRenameScript(oldUsername, newUsername)

	return &renameUsernameTask{
		BaseTask: taskrunner.NewBaseTask(
			taskrunner.WithName("Rename Username"),
			taskrunner.WithScript(script),
			taskrunner.WithTimeoutSeconds(120),
		),
		callback: renameCallbackData{
			ServerID:    server.ID,
			TeamID:      server.TeamID,
			OldUsername: oldUsername,
			NewUsername: newUsername,
		},
	}
}

// SetServerPlatformUpdateID sets the server platform update ID for callback tracking
func (t *renameUsernameTask) SetServerPlatformUpdateID(id string) {
	t.callback.ServerPlatformUpdateID = id
}

// TypeName returns the registered type name for reconstruction
func (t *renameUsernameTask) TypeName() string {
	return RenameUsernameTaskType
}

// MarshalPayload returns JSON representation of the callback data
func (t *renameUsernameTask) MarshalPayload() ([]byte, error) {
	return json.Marshal(t.callback)
}

// OnSuccess is called when the task completes successfully
func (t *renameUsernameTask) OnSuccess(ctx context.Context, cbCtx *taskrunner.CallbackContext, taskID string) error {
	if cbCtx.Logger != nil {
		cbCtx.Logger.Info().
			Str("task_id", taskID).
			Str("server_id", t.callback.ServerID).
			Msg("RenameUsername: completed successfully")
	}

	now := time.Now()

	// Update server_platform_updates status to completed
	if err := cbCtx.DB.
		Model(&models.ServerPlatformUpdate{}).
		Where("id = ?", t.callback.ServerPlatformUpdateID).
		Updates(map[string]interface{}{
			"status":       types.UpdateStatusCompleted,
			"completed_at": now,
			"task_id":      taskID,
		}).Error; err != nil {
		return fmt.Errorf("failed to update server platform update status: %w", err)
	}

	// Update server username
	if err := cbCtx.DB.
		Model(&servermodels.Server{}).
		Where("id = ?", t.callback.ServerID).
		Update("username", t.callback.NewUsername).Error; err != nil {
		return fmt.Errorf("failed to update server username: %w", err)
	}

	// Update all sites on this server: user and path
	oldPath := fmt.Sprintf("/home/%s/", t.callback.OldUsername)
	newPath := fmt.Sprintf("/home/%s/", t.callback.NewUsername)

	if err := cbCtx.DB.
		Model(&sitemodels.Site{}).
		Where("server_id = ? AND user = ?", t.callback.ServerID, t.callback.OldUsername).
		Updates(map[string]interface{}{
			"user": t.callback.NewUsername,
		}).Error; err != nil {
		if cbCtx.Logger != nil {
			cbCtx.Logger.Error().Err(err).Msg("Failed to update site users")
		}
	}

	// Update site paths using SQL REPLACE
	if err := cbCtx.DB.Exec(
		"UPDATE sites SET path = REPLACE(path, ?, ?) WHERE server_id = ? AND path LIKE ?",
		oldPath, newPath, t.callback.ServerID, oldPath+"%",
	).Error; err != nil {
		if cbCtx.Logger != nil {
			cbCtx.Logger.Error().Err(err).Msg("Failed to update site paths")
		}
	}

	// Broadcast completion
	cbCtx.BroadcastToTeam(t.callback.TeamID, "platform_update.status_changed", map[string]interface{}{
		"update_id": t.callback.ServerPlatformUpdateID,
		"server_id": t.callback.ServerID,
		"status":    "completed",
	})

	return nil
}

// OnFailure is called when the task fails
func (t *renameUsernameTask) OnFailure(ctx context.Context, cbCtx *taskrunner.CallbackContext, taskID string, exitCode int) error {
	if cbCtx.Logger != nil {
		cbCtx.Logger.Error().
			Str("task_id", taskID).
			Str("server_id", t.callback.ServerID).
			Int("exit_code", exitCode).
			Msg("RenameUsername: failed")
	}

	// Get task output for error message
	output := cbCtx.GetTaskOutputTail(taskID, 10)

	// Update server_platform_updates status to failed
	if err := cbCtx.DB.
		Model(&models.ServerPlatformUpdate{}).
		Where("id = ?", t.callback.ServerPlatformUpdateID).
		Updates(map[string]interface{}{
			"status":        types.UpdateStatusFailed,
			"error_message": output,
			"task_id":       taskID,
		}).Error; err != nil {
		return fmt.Errorf("failed to update server platform update status: %w", err)
	}

	// Broadcast failure
	cbCtx.BroadcastToTeam(t.callback.TeamID, "platform_update.status_changed", map[string]interface{}{
		"update_id": t.callback.ServerPlatformUpdateID,
		"server_id": t.callback.ServerID,
		"status":    "failed",
	})

	return nil
}

// OnExpired is called when the task times out
func (t *renameUsernameTask) OnExpired(ctx context.Context, cbCtx *taskrunner.CallbackContext, taskID string) error {
	if cbCtx.Logger != nil {
		cbCtx.Logger.Error().
			Str("task_id", taskID).
			Str("server_id", t.callback.ServerID).
			Msg("RenameUsername: timed out")
	}

	if err := cbCtx.DB.
		Model(&models.ServerPlatformUpdate{}).
		Where("id = ?", t.callback.ServerPlatformUpdateID).
		Updates(map[string]interface{}{
			"status":        types.UpdateStatusFailed,
			"error_message": "Task timed out",
			"task_id":       taskID,
		}).Error; err != nil {
		return fmt.Errorf("failed to update server platform update status: %w", err)
	}

	cbCtx.BroadcastToTeam(t.callback.TeamID, "platform_update.status_changed", map[string]interface{}{
		"update_id": t.callback.ServerPlatformUpdateID,
		"server_id": t.callback.ServerID,
		"status":    "failed",
	})

	return nil
}

// NewTask implements taskrunner.CallbackStateFactory
func (s renameCallbackData) NewTask() taskrunner.CallbackHandler {
	return &renameUsernameTask{
		BaseTask: taskrunner.NewBaseTask(),
		callback: s,
	}
}

// buildRenameScript renders the rename username bash script
func buildRenameScript(oldUsername, newUsername string) string {
	content, err := templates.FS.ReadFile("rename_username.sh")
	if err != nil {
		panic(fmt.Sprintf("failed to read rename_username.sh template: %v", err))
	}

	tmpl, err := template.New("rename_username").Parse(string(content))
	if err != nil {
		panic(fmt.Sprintf("failed to parse rename_username.sh template: %v", err))
	}

	var buf bytes.Buffer
	data := struct {
		OldUsername string
		NewUsername string
	}{
		OldUsername: oldUsername,
		NewUsername: newUsername,
	}

	if err := tmpl.Execute(&buf, data); err != nil {
		panic(fmt.Sprintf("failed to execute rename_username.sh template: %v", err))
	}

	return buf.String()
}
