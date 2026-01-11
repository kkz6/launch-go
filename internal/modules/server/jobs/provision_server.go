package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/enums"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeProvisionServer = "server:provision"

// ProvisionServerPayload contains data for provisioning a server
type ProvisionServerPayload struct {
	ServerID  string   `json:"server_id"`
	TeamID    string   `json:"team_id"`
	SshKeyIDs []string `json:"ssh_key_ids,omitempty"`
}

// ProvisionServerJob handles server provisioning
type ProvisionServerJob struct {
	*JobContext
}

// NewProvisionServerTask creates a new asynq task for provisioning a server
func NewProvisionServerTask(serverID, teamID string, sshKeyIDs []string) (*asynq.Task, error) {
	return jobs.NewTask(TypeProvisionServer, ProvisionServerPayload{
		ServerID:  serverID,
		TeamID:    teamID,
		SshKeyIDs: sshKeyIDs,
	})
}

// Handle processes the provision server job
func (j *ProvisionServerJob) Handle(ctx context.Context, t *asynq.Task) error {
	payload, err := jobs.UnmarshalPayload[ProvisionServerPayload](t)
	if err != nil {
		return err
	}

	j.Logger.Info().
		Str("server_id", payload.ServerID).
		Str("team_id", payload.TeamID).
		Int("ssh_keys", len(payload.SshKeyIDs)).
		Msg("Starting server provisioning")

	// Fetch the server
	server, err := j.FindServer(ctx, payload.ServerID)
	if err != nil {
		return err
	}

	// Update server status to provisioning
	if err := j.DB.Model(server).Update("status", enums.ServerStatusProvisioning).Error; err != nil {
		return fmt.Errorf("failed to update server status: %w", err)
	}

	j.broadcastProgress(payload.ServerID, "provisioning", "Starting server provisioning...")

	// Fetch SSH keys if provided
	var sshKeys []models.SshKey
	if len(payload.SshKeyIDs) > 0 {
		if err := j.DB.Where("id IN ?", payload.SshKeyIDs).Find(&sshKeys).Error; err != nil {
			j.Logger.Warn().Err(err).Msg("Failed to fetch SSH keys")
		}
	}

	// TODO: Run the actual provisioning task
	// _, err = j.RunTask(server, tasks.NewProvisionServer(server)).
	//     AsRoot().
	//     KeepTrack().
	//     Dispatch(ctx)
	// if err != nil {
	//     return err
	// }

	_ = sshKeys // use when implementing full provisioning

	j.broadcastProgress(payload.ServerID, "active", "Server provisioned successfully")

	return nil
}

// Failed handles job failure
func (j *ProvisionServerJob) Failed(ctx context.Context, t *asynq.Task, err error) {
	payload, unmarshalErr := jobs.UnmarshalPayload[ProvisionServerPayload](t)
	if unmarshalErr != nil {
		j.Logger.Error().Err(unmarshalErr).Msg("Failed to unmarshal payload in failure handler")
		return
	}

	j.Logger.Error().
		Err(err).
		Str("server_id", payload.ServerID).
		Msg("Server provisioning failed")

	// Update server status to failed
	j.DB.Model(&models.Server{}).
		Where("id = ?", payload.ServerID).
		Update("status", enums.ServerStatusFailed)

	j.broadcastProgress(payload.ServerID, "failed", "Server provisioning failed")
}

func (j *ProvisionServerJob) broadcastProgress(serverID, status, message string) {
	j.BroadcastToServer(serverID, "server.progress", map[string]interface{}{
		"server_id": serverID,
		"status":    status,
		"message":   message,
	})
}
