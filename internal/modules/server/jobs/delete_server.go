package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeDeleteServer = "server:delete"

// DeleteServerPayload contains data for deleting a server
type DeleteServerPayload struct {
	ServerID string  `json:"server_id"`
	TeamID   string  `json:"team_id"`
	UserID   *string `json:"user_id,omitempty"`
}

// DeleteServerJob handles server deletion operations
type DeleteServerJob struct {
	*JobContext
}

// NewDeleteServerTask creates a new asynq task for deleting a server
func NewDeleteServerTask(serverID, teamID string, userID *string) (*asynq.Task, error) {
	return jobs.NewTask(TypeDeleteServer, DeleteServerPayload{
		ServerID: serverID,
		TeamID:   teamID,
		UserID:   userID,
	})
}

// Handle processes the delete server job
func (j *DeleteServerJob) Handle(ctx context.Context, t *asynq.Task) error {
	payload, err := jobs.UnmarshalPayload[DeleteServerPayload](t)
	if err != nil {
		return err
	}

	j.Logger.Info().
		Str("server_id", payload.ServerID).
		Str("team_id", payload.TeamID).
		Msg("Deleting server")

	// Fetch the server
	server, err := j.FindServer(ctx, payload.ServerID)
	if err != nil {
		return err
	}

	j.broadcastProgress(payload.ServerID, "deleting", fmt.Sprintf("Deleting server: %s", server.Name))

	// TODO: Implement actual server deletion:
	// 1. Delete server from cloud provider (if managed)
	// 2. Clean up associated resources (sites, databases, etc.)

	// Delete the server record
	if err := j.DB.Delete(server).Error; err != nil {
		return fmt.Errorf("failed to delete server record: %w", err)
	}

	j.broadcastProgress(payload.ServerID, "deleted", fmt.Sprintf("Server %s deleted successfully", server.Name))

	return nil
}

// Failed handles job failure
func (j *DeleteServerJob) Failed(ctx context.Context, t *asynq.Task, err error) {
	payload, unmarshalErr := jobs.UnmarshalPayload[DeleteServerPayload](t)
	if unmarshalErr != nil {
		j.Logger.Error().Err(unmarshalErr).Msg("Failed to unmarshal payload in failure handler")
		return
	}

	j.Logger.Error().
		Err(err).
		Str("server_id", payload.ServerID).
		Msg("Failed to delete server")

	j.broadcastProgress(payload.ServerID, "failed", "Failed to delete server")
}

func (j *DeleteServerJob) broadcastProgress(serverID, status, message string) {
	j.BroadcastToServer(serverID, "server.progress", map[string]interface{}{
		"server_id": serverID,
		"status":    status,
		"message":   message,
		"timestamp": time.Now().Format(time.RFC3339),
	})
}
