package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeRebootServer = "server:reboot"

// RebootServerPayload contains data for rebooting a server
type RebootServerPayload struct {
	ServerID string  `json:"server_id"`
	UserID   *string `json:"user_id,omitempty"`
}

// RebootServerJob handles server reboot operations
type RebootServerJob struct {
	*JobContext
}

// NewRebootServerTask creates a new asynq task for rebooting a server
func NewRebootServerTask(serverID string, userID *string) (*asynq.Task, error) {
	return jobs.NewTask(TypeRebootServer, RebootServerPayload{
		ServerID: serverID,
		UserID:   userID,
	})
}

// Handle processes the reboot server job
func (j *RebootServerJob) Handle(ctx context.Context, t *asynq.Task) error {
	payload, err := jobs.UnmarshalPayload[RebootServerPayload](t)
	if err != nil {
		return err
	}

	j.Logger.Info().
		Str("server_id", payload.ServerID).
		Msg("Rebooting server")

	// Fetch the server
	server, err := j.FindServer(ctx, payload.ServerID)
	if err != nil {
		return err
	}

	j.broadcastProgress(payload.ServerID, "rebooting", fmt.Sprintf("Rebooting server: %s", server.Name))

	// TODO: Run the actual reboot task
	// _, err = j.RunTask(server, tasks.NewRebootServer()).
	//     AsRoot().
	//     Dispatch(ctx)
	// if err != nil {
	//     return err
	// }

	j.broadcastProgress(payload.ServerID, "rebooted", fmt.Sprintf("Server %s rebooted successfully", server.Name))

	return nil
}

// Failed handles job failure
func (j *RebootServerJob) Failed(ctx context.Context, t *asynq.Task, err error) {
	payload, unmarshalErr := jobs.UnmarshalPayload[RebootServerPayload](t)
	if unmarshalErr != nil {
		j.Logger.Error().Err(unmarshalErr).Msg("Failed to unmarshal payload in failure handler")
		return
	}

	j.Logger.Error().
		Err(err).
		Str("server_id", payload.ServerID).
		Msg("Failed to reboot server")

	j.broadcastProgress(payload.ServerID, "failed", "Failed to reboot server")
}

func (j *RebootServerJob) broadcastProgress(serverID, status, message string) {
	j.BroadcastToServer(serverID, "server.progress", map[string]interface{}{
		"server_id": serverID,
		"status":    status,
		"message":   message,
		"timestamp": time.Now().Format(time.RFC3339),
	})
}
