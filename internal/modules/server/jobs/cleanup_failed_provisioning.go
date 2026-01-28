package jobs

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/types"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	"github.com/kkz6/launch-go/internal/pkg/launch/activity"
)

const TypeCleanupFailedProvisioning = "server:cleanup_failed_provisioning"

type CleanupFailedProvisioningPayload struct {
	ServerID         string  `json:"server_id"`
	TeamID           string  `json:"team_id"`
	UserID           *string `json:"user_id,omitempty"`
	ServerProviderID string  `json:"server_provider_id,omitempty"`
	Reason           string  `json:"reason,omitempty"`
	DeleteRecord     bool    `json:"delete_record,omitempty"`
}

// CleanupFailedProvisioningJob cleans up resources after a failed server provisioning.
// This job is triggered when WaitForServerToConnect or ProvisionServer fails.
// It removes the server from the cloud provider and optionally deletes the database record.
type CleanupFailedProvisioningJob struct {
	Deps    *JobDeps
	Payload CleanupFailedProvisioningPayload

	server *models.Server
}

func NewCleanupFailedProvisioningJob(p CleanupFailedProvisioningPayload) pkgjobs.Handler {
	return &CleanupFailedProvisioningJob{Deps: deps, Payload: p}
}

func (j *CleanupFailedProvisioningJob) Handle(ctx context.Context) error {
	var err error

	j.server, err = j.Deps.Repos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("find server: %w", err)
	}

	j.Deps.Logger.Info().
		Str("server_id", j.server.ID).
		Str("server_name", j.server.Name).
		Str("reason", j.Payload.Reason).
		Msg("starting cleanup of failed provisioning")

	if j.server.Status != types.ServerStatusFailed {
		if err := j.Deps.Repos.Server().UpdateStatus(ctx, j.server.ID, types.ServerStatusFailed); err != nil {
			j.Deps.Logger.Error().Err(err).
				Msg("failed to update server status to failed")
		}
	}

	providerServerID := ""
	if j.server.ProviderData != nil {
		if id, ok := j.server.ProviderData["provider_server_id"].(string); ok {
			providerServerID = id
		}
	}

	if j.server.Provider != types.ProviderCustom && providerServerID != "" {
		if err := j.cleanupProviderResources(ctx, providerServerID); err != nil {
			j.Deps.Logger.Error().Err(err).
				Str("server_id", j.server.ID).
				Str("provider", j.server.Provider.String()).
				Msg("failed to cleanup provider resources")
		}
	}

	if j.server.ProviderData != nil {
		if sshKeyID, ok := j.server.ProviderData["ssh_key_id"].(string); ok && sshKeyID != "" {
			if err := j.cleanupProviderSSHKey(ctx, sshKeyID); err != nil {
				j.Deps.Logger.Error().Err(err).
					Str("server_id", j.server.ID).
					Str("ssh_key_id", sshKeyID).
					Msg("failed to cleanup provider SSH key")
			}
		}
	}

	activity.RecordWithLogAndPropsPtr(ctx, "server", "provisioning_failed", j.Payload.UserID, j.server, "Server provisioning failed and resources were cleaned up", map[string]any{
		"reason": j.Payload.Reason,
	})

	j.Deps.BroadcastServerEvent(j.server, "server.provisioning_cleanup_complete", map[string]any{
		"server_id": j.server.ID,
		"reason":    j.Payload.Reason,
		"deleted":   j.Payload.DeleteRecord,
	})

	if j.Payload.DeleteRecord {
		if err := j.Deps.Repos.Server().Delete(ctx, j.server.ID); err != nil {
			return fmt.Errorf("delete server record: %w", err)
		}

		j.Deps.Logger.Info().
			Str("server_id", j.server.ID).
			Msg("server record deleted after failed provisioning")

		j.Deps.BroadcastServerEvent(j.server, "server.deleted", map[string]any{
			"server_id": j.server.ID,
			"reason":    "provisioning_failed",
		})
	} else {
		if err := j.Deps.Repos.Server().UpdateFields(ctx, j.server.ID, map[string]any{
			"connected": false,
		}); err != nil {
			j.Deps.Logger.Error().Err(err).
				Msg("failed to update server fields")
		}

		j.Deps.Logger.Info().
			Str("server_id", j.server.ID).
			Msg("server marked as failed, record preserved for debugging")
	}

	return nil
}

func (j *CleanupFailedProvisioningJob) cleanupProviderResources(ctx context.Context, providerServerID string) error {
	provider, err := j.Deps.ProviderFactory.Create(j.server.Provider)
	if err != nil {
		return fmt.Errorf("create provider: %w", err)
	}

	credentials, err := j.getProviderCredentials(ctx)
	if err != nil {
		return fmt.Errorf("get credentials: %w", err)
	}

	if len(credentials) == 0 {
		return fmt.Errorf("no credentials found for server provider")
	}

	j.Deps.Logger.Info().
		Str("server_id", j.server.ID).
		Str("provider", j.server.Provider.String()).
		Str("provider_server_id", providerServerID).
		Msg("deleting server from cloud provider")

	if err := provider.Delete(ctx, j.server, credentials); err != nil {
		return fmt.Errorf("delete server from provider: %w", err)
	}

	j.Deps.Logger.Info().
		Str("server_id", j.server.ID).
		Str("provider", j.server.Provider.String()).
		Msg("server deleted from cloud provider")

	return nil
}

func (j *CleanupFailedProvisioningJob) cleanupProviderSSHKey(ctx context.Context, sshKeyID string) error {
	provider, err := j.Deps.ProviderFactory.Create(j.server.Provider)
	if err != nil {
		return fmt.Errorf("create provider: %w", err)
	}

	credentials, err := j.getProviderCredentials(ctx)
	if err != nil {
		return fmt.Errorf("get credentials: %w", err)
	}

	if len(credentials) == 0 {
		return fmt.Errorf("no credentials found for server provider")
	}

	j.Deps.Logger.Info().
		Str("server_id", j.server.ID).
		Str("ssh_key_id", sshKeyID).
		Msg("deleting SSH key from cloud provider")

	if deleter, ok := provider.(interface {
		DeleteSSHKey(ctx context.Context, keyID string, credentials map[string]any) error
	}); ok {
		if err := deleter.DeleteSSHKey(ctx, sshKeyID, credentials); err != nil {
			return fmt.Errorf("delete SSH key: %w", err)
		}
		j.Deps.Logger.Info().
			Str("ssh_key_id", sshKeyID).
			Msg("SSH key deleted from cloud provider")
	}

	return nil
}

func (j *CleanupFailedProvisioningJob) getProviderCredentials(ctx context.Context) (map[string]any, error) {
	serverProviderID := j.Payload.ServerProviderID
	if serverProviderID == "" && j.server.ServerProviderID != nil {
		serverProviderID = *j.server.ServerProviderID
	}

	if serverProviderID == "" {
		return nil, fmt.Errorf("no server provider ID")
	}

	serverProvider, err := j.Deps.Repos.ServerProvider().FindByID(ctx, serverProviderID)
	if err != nil {
		return nil, fmt.Errorf("find server provider: %w", err)
	}

	var credentials map[string]any
	credStr := serverProvider.Credentials.String()
	if credStr != "" {
		if err := json.Unmarshal([]byte(credStr), &credentials); err != nil {
			return nil, fmt.Errorf("parse credentials: %w", err)
		}
	}

	return credentials, nil
}

func (j *CleanupFailedProvisioningJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).
		Str("server_id", j.Payload.ServerID).
		Str("reason", j.Payload.Reason).
		Msg("failed to cleanup failed provisioning")

	_ = j.Deps.Repos.Server().UpdateStatus(ctx, j.Payload.ServerID, types.ServerStatusFailed)

	server, findErr := j.Deps.Repos.Server().FindByID(ctx, j.Payload.ServerID)
	if findErr == nil {
		j.Deps.BroadcastServerEvent(server, "server.cleanup_failed", map[string]any{
			"server_id": j.Payload.ServerID,
			"error":     err.Error(),
		})
	}
}

func NewCleanupFailedProvisioningTask(serverID, teamID string, serverProviderID string, userID *string, reason string, deleteRecord bool) (*asynq.Task, error) {
	return pkgjobs.TaskWithID(TypeCleanupFailedProvisioning,
		CleanupFailedProvisioningPayload{
			ServerID:         serverID,
			TeamID:           teamID,
			ServerProviderID: serverProviderID,
			UserID:           userID,
			Reason:           reason,
			DeleteRecord:     deleteRecord,
		},
		pkgjobs.Dedup("cleanup_provisioning", serverID),
	)
}
