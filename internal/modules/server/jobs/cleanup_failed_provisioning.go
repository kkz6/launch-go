package jobs

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/enums"
	"github.com/kkz6/launch-go/internal/pkg/activity"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
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
	ctx     *JobContext
	Payload CleanupFailedProvisioningPayload
}

func (j *CleanupFailedProvisioningJob) Handle(ctx context.Context) error {
	server, err := j.ctx.Repos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	j.ctx.LogInfo("Starting cleanup of failed provisioning",
		"server_id", server.ID,
		"server_name", server.Name,
		"reason", j.Payload.Reason,
	)

	// Update server status to failed if not already
	if server.Status != enums.ServerStatusFailed {
		if err := j.ctx.Repos.Server().UpdateStatus(ctx, server.ID, enums.ServerStatusFailed); err != nil {
			j.ctx.LogError(err, "Failed to update server status to failed")
		}
	}

	// Get provider server ID from provider data
	providerServerID := ""
	if server.ProviderData != nil {
		if id, ok := server.ProviderData["provider_server_id"].(string); ok {
			providerServerID = id
		}
	}

	// Clean up resources on cloud provider
	if server.Provider != enums.ProviderCustom && providerServerID != "" {
		if err := j.cleanupProviderResources(ctx, server, providerServerID); err != nil {
			j.ctx.LogError(err, "Failed to cleanup provider resources",
				"server_id", server.ID,
				"provider", server.Provider.String(),
			)
			// Continue with the rest of the cleanup
		}
	}

	// Clean up SSH key on provider if it was created
	if server.ProviderData != nil {
		if sshKeyID, ok := server.ProviderData["ssh_key_id"].(string); ok && sshKeyID != "" {
			if err := j.cleanupProviderSSHKey(ctx, server, sshKeyID); err != nil {
				j.ctx.LogError(err, "Failed to cleanup provider SSH key",
					"server_id", server.ID,
					"ssh_key_id", sshKeyID,
				)
			}
		}
	}

	// Log activity
	logger := activity.New(j.ctx.DB).
		WithContext(ctx).
		UseLog("server").
		On(server).
		WithEvent("provisioning_failed").
		WithProperty("reason", j.Payload.Reason)
	if j.Payload.UserID != nil {
		logger.CausedByUser(*j.Payload.UserID)
	}
	logger.Log("Server provisioning failed and resources were cleaned up")

	// Broadcast failure event
	j.ctx.BroadcastServerEvent(server, "server.provisioning_cleanup_complete", map[string]any{
		"server_id": server.ID,
		"reason":    j.Payload.Reason,
		"deleted":   j.Payload.DeleteRecord,
	})

	// Optionally delete the server record
	if j.Payload.DeleteRecord {
		if err := j.ctx.Repos.Server().Delete(ctx, server.ID); err != nil {
			return fmt.Errorf("failed to delete server record: %w", err)
		}

		j.ctx.LogInfo("Server record deleted after failed provisioning",
			"server_id", server.ID,
		)

		j.ctx.BroadcastServerEvent(server, "server.deleted", map[string]any{
			"server_id": server.ID,
			"reason":    "provisioning_failed",
		})
	} else {
		// Update server with failure information
		if err := j.ctx.Repos.Server().UpdateFields(ctx, server.ID, map[string]any{
			"is_connected": false,
		}); err != nil {
			j.ctx.LogError(err, "Failed to update server fields")
		}

		j.ctx.LogInfo("Server marked as failed, record preserved for debugging",
			"server_id", server.ID,
		)
	}

	return nil
}

// cleanupProviderResources deletes the server instance from the cloud provider
func (j *CleanupFailedProvisioningJob) cleanupProviderResources(ctx context.Context, server any, providerServerID string) error {
	srv, err := j.ctx.Repos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return err
	}

	provider, err := j.ctx.ProviderFactory.Create(srv.Provider)
	if err != nil {
		return fmt.Errorf("failed to create provider: %w", err)
	}

	// Get credentials
	credentials, err := j.getProviderCredentials(ctx, srv)
	if err != nil {
		return fmt.Errorf("failed to get credentials: %w", err)
	}

	if credentials == nil || len(credentials) == 0 {
		return fmt.Errorf("no credentials found for server provider")
	}

	j.ctx.LogInfo("Deleting server from cloud provider",
		"server_id", srv.ID,
		"provider", srv.Provider.String(),
		"provider_server_id", providerServerID,
	)

	if err := provider.Delete(ctx, srv, credentials); err != nil {
		return fmt.Errorf("failed to delete server from provider: %w", err)
	}

	j.ctx.LogInfo("Server deleted from cloud provider",
		"server_id", srv.ID,
		"provider", srv.Provider.String(),
	)

	return nil
}

// cleanupProviderSSHKey removes the SSH key that was created on the provider
func (j *CleanupFailedProvisioningJob) cleanupProviderSSHKey(ctx context.Context, server any, sshKeyID string) error {
	srv, err := j.ctx.Repos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return err
	}

	provider, err := j.ctx.ProviderFactory.Create(srv.Provider)
	if err != nil {
		return fmt.Errorf("failed to create provider: %w", err)
	}

	credentials, err := j.getProviderCredentials(ctx, srv)
	if err != nil {
		return fmt.Errorf("failed to get credentials: %w", err)
	}

	if credentials == nil || len(credentials) == 0 {
		return fmt.Errorf("no credentials found for server provider")
	}

	j.ctx.LogInfo("Deleting SSH key from cloud provider",
		"server_id", srv.ID,
		"ssh_key_id", sshKeyID,
	)

	// Try to delete the SSH key - this method may not exist on all providers
	if deleter, ok := provider.(interface {
		DeleteSSHKey(ctx context.Context, keyID string, credentials map[string]any) error
	}); ok {
		if err := deleter.DeleteSSHKey(ctx, sshKeyID, credentials); err != nil {
			return fmt.Errorf("failed to delete SSH key: %w", err)
		}
		j.ctx.LogInfo("SSH key deleted from cloud provider",
			"ssh_key_id", sshKeyID,
		)
	}

	return nil
}

// getProviderCredentials retrieves the credentials for the server provider
func (j *CleanupFailedProvisioningJob) getProviderCredentials(ctx context.Context, server any) (map[string]any, error) {
	srv, err := j.ctx.Repos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return nil, err
	}

	serverProviderID := j.Payload.ServerProviderID
	if serverProviderID == "" && srv.ServerProviderID != nil {
		serverProviderID = *srv.ServerProviderID
	}

	if serverProviderID == "" {
		return nil, fmt.Errorf("no server provider ID")
	}

	serverProvider, err := j.ctx.Repos.ServerProvider().FindByID(ctx, serverProviderID)
	if err != nil {
		return nil, fmt.Errorf("failed to find server provider: %w", err)
	}

	var credentials map[string]any
	credStr := serverProvider.Credentials.String()
	if credStr != "" {
		if err := json.Unmarshal([]byte(credStr), &credentials); err != nil {
			return nil, fmt.Errorf("failed to parse credentials: %w", err)
		}
	}

	return credentials, nil
}

func (j *CleanupFailedProvisioningJob) Failed(ctx context.Context, err error) {
	j.ctx.LogError(err, "Failed to cleanup failed provisioning",
		"server_id", j.Payload.ServerID,
		"reason", j.Payload.Reason,
	)

	// Even if cleanup fails, ensure server is marked as failed
	_ = j.ctx.Repos.Server().UpdateStatus(ctx, j.Payload.ServerID, enums.ServerStatusFailed)

	server, findErr := j.ctx.Repos.Server().FindByID(ctx, j.Payload.ServerID)
	if findErr == nil {
		j.ctx.BroadcastServerEvent(server, "server.cleanup_failed", map[string]any{
			"server_id": j.Payload.ServerID,
			"error":     err.Error(),
		})
	}
}

// NewCleanupFailedProvisioningJob creates a new CleanupFailedProvisioningJob
func NewCleanupFailedProvisioningJob(ctx *JobContext, payload CleanupFailedProvisioningPayload) *CleanupFailedProvisioningJob {
	return &CleanupFailedProvisioningJob{
		ctx:     ctx,
		Payload: payload,
	}
}

// NewCleanupFailedProvisioningTask creates an asynq task for cleanup
// Uses TaskID for deduplication to prevent duplicate cleanup runs
func NewCleanupFailedProvisioningTask(serverID, teamID string, serverProviderID string, userID *string, reason string, deleteRecord bool) (*asynq.Task, error) {
	return pkgjobs.NewTask(TypeCleanupFailedProvisioning, CleanupFailedProvisioningPayload{
		ServerID:         serverID,
		TeamID:           teamID,
		ServerProviderID: serverProviderID,
		UserID:           userID,
		Reason:           reason,
		DeleteRecord:     deleteRecord,
	}, asynq.TaskID(fmt.Sprintf("cleanup_provisioning:%s", serverID)))
}
