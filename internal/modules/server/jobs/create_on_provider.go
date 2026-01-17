package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/enums"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

const TypeCreateOnProvider = "server:create_on_provider"

type CreateOnProviderPayload struct {
	ServerID         string   `json:"server_id"`
	TeamID           string   `json:"team_id"`
	UserID           *string  `json:"user_id,omitempty"`
	SSHKeyIDs        []string `json:"ssh_key_ids,omitempty"`
	ServerProviderID string   `json:"server_provider_id,omitempty"`
}

// CreateOnProviderJob creates a server on the cloud provider.
// This job handles the API call to create the server instance.
type CreateOnProviderJob struct {
	ctx     *JobContext
	Payload CreateOnProviderPayload
}

// NewCreateOnProviderJob creates a new CreateOnProviderJob with the given context and payload.
func NewCreateOnProviderJob(ctx *JobContext, payload CreateOnProviderPayload) *CreateOnProviderJob {
	return &CreateOnProviderJob{
		ctx:     ctx,
		Payload: payload,
	}
}

func (j *CreateOnProviderJob) Handle(ctx context.Context) error {
	server, err := j.ctx.Repos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	if server.Provider == enums.ProviderCustom {
		j.ctx.LogInfo("Skipping cloud creation for custom server", "server_id", server.ID)
		return nil
	}

	provider, err := j.ctx.ProviderFactory.Create(server.Provider)
	if err != nil {
		return fmt.Errorf("failed to create provider: %w", err)
	}

	var credentials map[string]any
	if j.Payload.ServerProviderID != "" {
		serverProvider, err := j.ctx.Repos.ServerProvider().FindByID(ctx, j.Payload.ServerProviderID)
		if err != nil {
			return fmt.Errorf("failed to find server provider: %w", err)
		}
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

	if err := j.ctx.Repos.Server().UpdateStatus(ctx, server.ID, enums.ServerStatusStarting); err != nil {
		return fmt.Errorf("failed to update server status: %w", err)
	}

	j.ctx.LogInfo("Creating server on provider",
		"server_id", server.ID,
		"provider", server.Provider.String(),
	)

	result, err := provider.Create(ctx, server, credentials)
	if err != nil {
		return fmt.Errorf("failed to create server on provider: %w", err)
	}

	j.ctx.LogInfo("Server created on provider",
		"server_id", server.ID,
		"provider_server_id", result.ProviderServerID,
	)

	providerData := server.ProviderData
	if providerData == nil {
		providerData = make(map[string]any)
	}
	providerData["provider_server_id"] = result.ProviderServerID
	if result.SSHKeyID != "" {
		providerData["ssh_key_id"] = result.SSHKeyID
	}
	for k, v := range result.ProviderData {
		providerData[k] = v
	}

	updates := map[string]any{
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

	if err := j.ctx.Repos.Server().UpdateFields(ctx, server.ID, updates); err != nil {
		return fmt.Errorf("failed to update server with provider data: %w", err)
	}

	if result.PublicIPv4 == "" {
		ip, err := j.waitForPublicIP(ctx, server, provider, credentials)
		if err != nil {
			j.ctx.LogError(err, "Failed to get public IP, will retry during provisioning")
		} else {
			result.PublicIPv4 = ip
		}
	}

	if result.PublicIPv4 != "" {
		if err := j.ctx.Repos.Server().UpdateFields(ctx, server.ID, map[string]any{
			"public_ipv4": result.PublicIPv4,
		}); err != nil {
			return fmt.Errorf("failed to update server IP: %w", err)
		}
	}

	j.ctx.BroadcastServerEvent(server, "server.created_on_provider", map[string]any{
		"server_id":          server.ID,
		"provider_server_id": result.ProviderServerID,
		"public_ipv4":        result.PublicIPv4,
	})

	j.ctx.LogInfo("Server created on provider, dispatching wait for connection job",
		"server_id", server.ID,
		"provider_server_id", result.ProviderServerID,
	)

	// Dispatch WaitForServerToConnect job to wait for SSH connectivity
	if err := j.dispatchWaitForConnection(); err != nil {
		return fmt.Errorf("failed to dispatch wait for connection job: %w", err)
	}

	return nil
}

// dispatchWaitForConnection dispatches the WaitForServerToConnect job
func (j *CreateOnProviderJob) dispatchWaitForConnection() error {
	if j.ctx.Queue == nil {
		return fmt.Errorf("queue client not available")
	}

	task, err := NewWaitForServerToConnectTask(
		j.Payload.ServerID,
		j.Payload.TeamID,
		j.Payload.ServerProviderID,
		j.Payload.UserID,
		j.Payload.SSHKeyIDs,
	)
	if err != nil {
		return fmt.Errorf("failed to create wait for connection task: %w", err)
	}

	if _, err := j.ctx.Queue.Enqueue(task); err != nil {
		return fmt.Errorf("failed to enqueue wait for connection job: %w", err)
	}

	j.ctx.LogInfo("WaitForServerToConnect job dispatched",
		"server_id", j.Payload.ServerID,
	)

	return nil
}

func (j *CreateOnProviderJob) waitForPublicIP(
	ctx context.Context,
	server any,
	provider any,
	credentials map[string]any,
) (string, error) {
	srv, err := j.ctx.Repos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return "", err
	}

	prov, err := j.ctx.ProviderFactory.Create(srv.Provider)
	if err != nil {
		return "", err
	}

	maxAttempts := 30
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		ip, err := prov.GetPublicIPv4(ctx, srv, credentials)
		if err == nil && ip != "" {
			return ip, nil
		}

		j.ctx.LogInfo("Waiting for public IP",
			"server_id", srv.ID,
			"attempt", attempt,
		)

		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(10 * time.Second):
			srv, _ = j.ctx.Repos.Server().FindByID(ctx, j.Payload.ServerID)
		}
	}

	return "", fmt.Errorf("timeout waiting for public IP")
}

func (j *CreateOnProviderJob) Failed(ctx context.Context, err error) {
	j.ctx.LogError(err, "Failed to create server on provider",
		"server_id", j.Payload.ServerID,
	)

	if updateErr := j.ctx.Repos.Server().UpdateStatus(ctx, j.Payload.ServerID, enums.ServerStatusFailed); updateErr != nil {
		j.ctx.LogError(updateErr, "Failed to update server status to failed")
	}

	server, findErr := j.ctx.Repos.Server().FindByID(ctx, j.Payload.ServerID)
	if findErr == nil {
		j.ctx.BroadcastServerEvent(server, "server.create_failed", map[string]any{
			"server_id": j.Payload.ServerID,
			"error":     err.Error(),
		})
	}
}

// NewCreateOnProviderTask creates an asynq task for creating a server on the provider
// Uses TaskID for deduplication to prevent duplicate server creation
func NewCreateOnProviderTask(serverID, teamID string, serverProviderID string, userID *string, sshKeyIDs []string) (*asynq.Task, error) {
	return pkgjobs.NewTask(TypeCreateOnProvider, CreateOnProviderPayload{
		ServerID:         serverID,
		TeamID:           teamID,
		ServerProviderID: serverProviderID,
		UserID:           userID,
		SSHKeyIDs:        sshKeyIDs,
	}, asynq.TaskID(fmt.Sprintf("create_on_provider:%s", serverID)))
}
