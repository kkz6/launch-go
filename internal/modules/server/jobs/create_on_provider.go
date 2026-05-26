package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/providers"
	"github.com/kkz6/launch-go/internal/modules/server/types"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	"github.com/kkz6/launch-go/internal/pkg/retry"
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
	Deps    *JobDeps
	Payload CreateOnProviderPayload

	server *models.Server
}

func NewCreateOnProviderJob(p CreateOnProviderPayload) pkgjobs.Handler {
	return &CreateOnProviderJob{Deps: deps, Payload: p}
}

func (j *CreateOnProviderJob) Handle(ctx context.Context) error {
	var err error
	j.server, err = j.Deps.Repos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	if j.server.Provider == types.ProviderCustom {
		j.Deps.Logger.Info().Str("server_id", j.server.ID).Msg("skipping cloud creation for custom server")
		return nil
	}

	provider, err := j.Deps.ProviderFactory.Create(j.server.Provider)
	if err != nil {
		return fmt.Errorf("failed to create provider: %w", err)
	}

	var credentials map[string]any
	if j.Payload.ServerProviderID != "" {
		serverProvider, err := j.Deps.Repos.ServerProvider().FindByID(ctx, j.Payload.ServerProviderID)
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

	if len(credentials) == 0 {
		return fmt.Errorf("no credentials found for server provider")
	}

	if err := j.Deps.Repos.Server().UpdateStatus(ctx, j.server.ID, types.ServerStatusStarting); err != nil {
		return fmt.Errorf("failed to update server status: %w", err)
	}

	j.Deps.Logger.Info().
		Str("server_id", j.server.ID).
		Str("provider", j.server.Provider.String()).
		Msg("creating server on provider")

	result, err := provider.Create(ctx, j.server, credentials)
	if err != nil {
		return fmt.Errorf("failed to create server on provider: %w", err)
	}

	j.Deps.Logger.Info().
		Str("server_id", j.server.ID).
		Str("provider_server_id", result.ProviderServerID).
		Msg("server created on provider")

	providerData := j.server.ProviderData
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
		updates["public_key"] = dbtype.EncryptedString(result.PublicKey)
	}
	if result.PrivateKey != "" {
		updates["private_key"] = dbtype.EncryptedString(result.PrivateKey)
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

	if err := j.Deps.Repos.Server().UpdateFields(ctx, j.server.ID, updates); err != nil {
		return fmt.Errorf("failed to update server with provider data: %w", err)
	}

	if result.PublicIPv4 == "" {
		ip, err := j.waitForPublicIP(ctx, j.server, provider, credentials)
		if err != nil {
			j.Deps.Logger.Error().Err(err).Msg("failed to get public IP, will retry during provisioning")
		} else {
			result.PublicIPv4 = ip
		}
	}

	if result.PublicIPv4 != "" {
		if err := j.Deps.Repos.Server().UpdateFields(ctx, j.server.ID, map[string]any{
			"public_ipv4": result.PublicIPv4,
		}); err != nil {
			return fmt.Errorf("failed to update server IP: %w", err)
		}
	}

	j.Deps.BroadcastServerEvent(j.server, "server.created_on_provider", map[string]any{
		"server_id":          j.server.ID,
		"provider_server_id": result.ProviderServerID,
		"public_ipv4":        result.PublicIPv4,
	})

	j.Deps.Logger.Info().
		Str("server_id", j.server.ID).
		Str("provider_server_id", result.ProviderServerID).
		Msg("server created on provider, dispatching wait for connection job")

	// Dispatch WaitForServerToConnect job to wait for SSH connectivity
	if err := j.dispatchWaitForConnection(); err != nil {
		return fmt.Errorf("failed to dispatch wait for connection job: %w", err)
	}

	return nil
}

// dispatchWaitForConnection dispatches the WaitForServerToConnect job
func (j *CreateOnProviderJob) dispatchWaitForConnection() error {
	if j.Deps.Queue == nil {
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

	if err := j.Deps.DispatchTask(task); err != nil {
		return fmt.Errorf("failed to dispatch wait for connection job: %w", err)
	}

	j.Deps.Logger.Info().
		Str("server_id", j.Payload.ServerID).
		Msg("WaitForServerToConnect job dispatched")

	return nil
}

func (j *CreateOnProviderJob) waitForPublicIP(
	ctx context.Context,
	_ any, // server (unused, we reload it)
	_ any, // provider (unused, we recreate it)
	credentials map[string]any,
) (string, error) {
	cfg := retry.ServerConnectionRetry
	cfg.OnRetry = func(attempt int, _ error, _ time.Duration) {
		j.Deps.Logger.Info().
			Str("server_id", j.Payload.ServerID).
			Int("attempt", attempt).
			Msg("waiting for public IP")
	}

	return retry.WithBackoff(ctx, cfg, func() (string, error) {
		srv, err := j.Deps.Repos.Server().FindByID(ctx, j.Payload.ServerID)
		if err != nil {
			return "", err
		}

		prov, err := j.Deps.ProviderFactory.Create(srv.Provider)
		if err != nil {
			return "", err
		}

		ip, err := prov.GetPublicIPv4(ctx, srv, credentials)
		if err != nil {
			return "", err
		}
		if ip == "" {
			return "", fmt.Errorf("no public IP available yet")
		}

		return ip, nil
	})
}

func (j *CreateOnProviderJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).
		Str("server_id", j.Payload.ServerID).
		Msg("failed to create server on provider")

	if updateErr := j.Deps.Repos.Server().UpdateStatus(ctx, j.Payload.ServerID, types.ServerStatusFailed); updateErr != nil {
		j.Deps.Logger.Error().Err(updateErr).Msg("failed to update server status to failed")
	}

	// Persist a friendly, plain-language reason so the UI can show it without
	// leaking raw upstream payloads. Sentry / worker logs still hold the
	// developer-facing details from logUpstreamError.
	if j.server != nil {
		friendly := providers.FriendlyError(j.server.Provider, err)
		if persistErr := j.Deps.DB.Model(&models.Server{}).
			Where("id = ?", j.Payload.ServerID).
			Update("provision_error", friendly).Error; persistErr != nil {
			j.Deps.Logger.Error().Err(persistErr).Msg("failed to persist provision_error")
		}

		j.Deps.BroadcastServerEvent(j.server, "server.create_failed", map[string]any{
			"server_id":      j.Payload.ServerID,
			"error":          err.Error(),
			"friendly_error": friendly,
		})
	}
}

// NewCreateOnProviderTask creates an asynq task for creating a server on the provider
// Uses TaskID for deduplication to prevent duplicate server creation
func NewCreateOnProviderTask(serverID, teamID string, serverProviderID string, userID *string, sshKeyIDs []string) (*asynq.Task, error) {
	return pkgjobs.Task(TypeCreateOnProvider, CreateOnProviderPayload{
		ServerID:         serverID,
		TeamID:           teamID,
		ServerProviderID: serverProviderID,
		UserID:           userID,
		SSHKeyIDs:        sshKeyIDs,
	}, asynq.TaskID(pkgjobs.Dedup("create_on_provider", serverID)))
}
