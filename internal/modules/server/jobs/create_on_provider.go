package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/enums"
	"github.com/kkz6/launch-go/internal/pkg/jobs"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

// CreateOnProviderJob creates a server on the cloud provider.
// This job handles the API call to create the server instance.
type CreateOnProviderJob struct {
	ServerJobBase
	Payload CreateOnProviderPayload
}

// Type returns the job type identifier
func (j *CreateOnProviderJob) Type() string {
	return TypeCreateOnProvider
}

// Handle processes the job
func (j *CreateOnProviderJob) Handle(ctx context.Context) error {
	// Find the server
	server, err := j.Repo().FindServerByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	// Skip for custom servers
	if server.Provider == enums.ProviderCustom {
		j.LogInfo("Skipping cloud creation for custom server", "server_id", server.ID)
		return nil
	}

	// Get the cloud provider
	provider, err := j.ProviderFactory().Create(server.Provider)
	if err != nil {
		return fmt.Errorf("failed to create provider: %w", err)
	}

	// Get credentials from server provider record
	var credentials map[string]interface{}
	if j.Payload.ServerProviderID != "" {
		serverProvider, err := j.Repo().FindServerProviderByID(ctx, j.Payload.ServerProviderID)
		if err != nil {
			return fmt.Errorf("failed to find server provider: %w", err)
		}
		// Decrypt and parse credentials
		credStr := serverProvider.Credentials.String()
		if credStr != "" {
			if err := json.Unmarshal([]byte(credStr), &credentials); err != nil {
				return fmt.Errorf("failed to parse credentials: %w", err)
			}
		}
	}

	if credentials == nil || len(credentials) == 0 {
		return fmt.Errorf("no credentials found for server provider")
	}

	// Update status to starting (before cloud creation)
	if err := j.Repo().UpdateServerStatus(ctx, server.ID, enums.ServerStatusStarting); err != nil {
		return fmt.Errorf("failed to update server status: %w", err)
	}

	j.LogInfo("Creating server on provider",
		"server_id", server.ID,
		"provider", server.Provider.String(),
	)

	// Create the server on the provider
	result, err := provider.Create(ctx, server, credentials)
	if err != nil {
		return fmt.Errorf("failed to create server on provider: %w", err)
	}

	j.LogInfo("Server created on provider",
		"server_id", server.ID,
		"provider_server_id", result.ProviderServerID,
	)

	// Merge provider data with result data
	providerData := server.ProviderData
	if providerData == nil {
		providerData = make(map[string]interface{})
	}
	providerData["provider_server_id"] = result.ProviderServerID
	if result.SSHKeyID != "" {
		providerData["ssh_key_id"] = result.SSHKeyID
	}
	for k, v := range result.ProviderData {
		providerData[k] = v
	}

	// Update server with provider data
	updates := map[string]interface{}{
		"provider_data": providerData,
	}

	if result.PublicKey != "" {
		updates["public_key"] = basemodels.EncryptedString(result.PublicKey)
	}
	if result.PrivateKey != "" {
		updates["private_key"] = basemodels.EncryptedString(result.PrivateKey)
	}
	if result.CPUCores > 0 {
		updates["cpu_cores"] = result.CPUCores
	}
	if result.MemoryMB > 0 {
		updates["memory_in_mb"] = result.MemoryMB
	}
	if result.DiskGB > 0 {
		updates["storage_in_gb"] = result.DiskGB
	}

	if err := j.Repo().UpdateServerFields(ctx, server.ID, updates); err != nil {
		return fmt.Errorf("failed to update server with provider data: %w", err)
	}

	// Wait for public IP if not immediately available
	if result.PublicIPv4 == "" {
		ip, err := j.waitForPublicIP(ctx, server, provider, credentials)
		if err != nil {
			j.LogError(err, "Failed to get public IP, will retry during provisioning")
		} else {
			result.PublicIPv4 = ip
		}
	}

	if result.PublicIPv4 != "" {
		if err := j.Repo().UpdateServerFields(ctx, server.ID, map[string]interface{}{
			"public_ipv4": result.PublicIPv4,
		}); err != nil {
			return fmt.Errorf("failed to update server IP: %w", err)
		}
	}

	// Broadcast server created event
	j.BroadcastServerEvent(server.ID, "server.created_on_provider", map[string]interface{}{
		"server_id":          server.ID,
		"provider_server_id": result.ProviderServerID,
		"public_ipv4":        result.PublicIPv4,
	})

	// The provisioning job will be dispatched by the worker scheduler
	// after this job completes successfully. We just need to update the
	// status to indicate we're ready for provisioning.
	j.LogInfo("Server created on provider, ready for provisioning",
		"server_id", server.ID,
		"provider_server_id", result.ProviderServerID,
	)

	return nil
}

// waitForPublicIP polls the provider for the server's public IP
func (j *CreateOnProviderJob) waitForPublicIP(
	ctx context.Context,
	server interface{},
	provider interface{},
	credentials map[string]interface{},
) (string, error) {
	type serverWithProvider interface {
		GetPublicIPv4(ctx context.Context, server interface{}, credentials map[string]interface{}) (string, error)
	}

	// Reload server to get updated provider data
	srv, err := j.Repo().FindServerByID(ctx, j.Payload.ServerID)
	if err != nil {
		return "", err
	}

	prov, err := j.ProviderFactory().Create(srv.Provider)
	if err != nil {
		return "", err
	}

	// Poll for IP with timeout
	maxAttempts := 30
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		ip, err := prov.GetPublicIPv4(ctx, srv, credentials)
		if err == nil && ip != "" {
			return ip, nil
		}

		j.LogInfo("Waiting for public IP",
			"server_id", srv.ID,
			"attempt", attempt,
		)

		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(10 * time.Second):
			// Reload server for updated provider_data
			srv, _ = j.Repo().FindServerByID(ctx, j.Payload.ServerID)
		}
	}

	return "", fmt.Errorf("timeout waiting for public IP")
}

// Failed is called when the job fails after all retries
func (j *CreateOnProviderJob) Failed(ctx context.Context, err error) {
	j.LogError(err, "Failed to create server on provider",
		"server_id", j.Payload.ServerID,
	)

	// Update server status to failed
	if updateErr := j.Repo().UpdateServerStatus(ctx, j.Payload.ServerID, enums.ServerStatusFailed); updateErr != nil {
		j.LogError(updateErr, "Failed to update server status to failed")
	}

	// Broadcast failure
	j.BroadcastServerEvent(j.Payload.ServerID, "server.create_failed", map[string]interface{}{
		"server_id": j.Payload.ServerID,
		"error":     err.Error(),
	})
}

// NewCreateOnProviderTask creates an asynq task for creating a server on the provider
func NewCreateOnProviderTask(serverID, teamID string, serverProviderID string, userID *string, sshKeyIDs []string) (*asynq.Task, error) {
	return jobs.NewTask(TypeCreateOnProvider, CreateOnProviderPayload{
		ServerID:         serverID,
		TeamID:           teamID,
		ServerProviderID: serverProviderID,
		UserID:           userID,
		SSHKeyIDs:        sshKeyIDs,
	})
}
