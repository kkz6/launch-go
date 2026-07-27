package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/modules/server/types"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	"github.com/kkz6/launch-go/internal/pkg/retry"
)

const TypeWaitForServerToConnect = "server:wait_for_connect"

// connectionTimeout is the timeout for individual SSH connection attempts
const connectionTimeout = 15 * time.Second

type WaitForServerToConnectPayload struct {
	ServerID         string   `json:"server_id"`
	TeamID           string   `json:"team_id"`
	UserID           *string  `json:"user_id,omitempty"`
	SSHKeyIDs        []string `json:"ssh_key_ids,omitempty"`
	ServerProviderID string   `json:"server_provider_id,omitempty"`
}

// WaitForServerToConnectJob waits for a server to become accessible via SSH.
// Once connected, it dispatches the ProvisionServer job.
type WaitForServerToConnectJob struct {
	Deps    *JobDeps
	Payload WaitForServerToConnectPayload

	server *models.Server
}

func NewWaitForServerToConnectJob(p WaitForServerToConnectPayload) pkgjobs.Handler {
	return &WaitForServerToConnectJob{Deps: deps, Payload: p}
}

func (j *WaitForServerToConnectJob) Handle(ctx context.Context) error {
	var err error
	j.server, err = j.Deps.Repos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	// Update status to indicate we're waiting for connection
	if err := j.Deps.Repos.Server().UpdateStatus(ctx, j.server.ID, types.ServerStatusStarting); err != nil {
		return fmt.Errorf("failed to update server status: %w", err)
	}

	j.Deps.BroadcastServerEvent(j.server, "server.waiting_for_connection", map[string]any{
		"server_id": j.server.ID,
		"status":    "waiting_for_connection",
	})

	j.Deps.Logger.Info().
		Str("server_id", j.server.ID).
		Str("server_name", j.server.Name).
		Msg("Waiting for server to become accessible")

	// Wait for server to have an IP address
	if err = j.waitForIP(ctx); err != nil {
		return fmt.Errorf("failed to get server IP: %w", err)
	}

	// Reload server with IP
	j.server, err = j.Deps.Repos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to reload server: %w", err)
	}

	// Attempt to connect with retry logic
	connected, attempt := j.attemptConnection(ctx)
	if !connected {
		return fmt.Errorf("server failed to become accessible after %d attempts", attempt)
	}

	// Update connectivity status
	if err := j.Deps.Repos.Server().UpdateFields(ctx, j.server.ID, map[string]any{
		"connected": true,
	}); err != nil {
		j.Deps.Logger.Error().Err(err).Msg("Failed to update connectivity status")
	}

	j.Deps.Logger.Info().
		Str("server_id", j.server.ID).
		Int("attempts", attempt).
		Msg("Server is now accessible, dispatching provisioning job")

	j.Deps.BroadcastServerEvent(j.server, "server.connected", map[string]any{
		"server_id": j.server.ID,
		"status":    "connected",
		"attempts":  attempt,
	})

	// Dispatch the ProvisionServer job
	if err := j.dispatchProvisionJob(); err != nil {
		return fmt.Errorf("failed to dispatch provision job: %w", err)
	}

	return nil
}

// waitForIP waits for the server to have a public IP address
func (j *WaitForServerToConnectJob) waitForIP(ctx context.Context) error {
	cfg := retry.ServerConnectionRetry
	cfg.OnRetry = func(attempt int, _ error, _ time.Duration) {
		j.Deps.Logger.Info().
			Str("server_id", j.Payload.ServerID).
			Int("attempt", attempt).
			Msg("Waiting for server IP address")
	}

	return retry.Do(ctx, cfg, func() error {
		server, err := j.Deps.Repos.Server().FindByID(ctx, j.Payload.ServerID)
		if err != nil {
			return err
		}

		if server.PublicIPv4 != nil && *server.PublicIPv4 != "" {
			return nil
		}

		return fmt.Errorf("server has no public IP yet")
	})
}

// attemptConnection tries to connect to the server with exponential backoff
func (j *WaitForServerToConnectJob) attemptConnection(ctx context.Context) (bool, int) {
	cfg := retry.ServerConnectionRetry
	cfg.OnRetry = func(attempt int, _ error, delay time.Duration) {
		// Reload server for event broadcast
		server, err := j.Deps.Repos.Server().FindByID(ctx, j.Payload.ServerID)
		if err != nil {
			return
		}

		j.Deps.Logger.Info().
			Str("server_id", server.ID).
			Int("attempt", attempt).
			Int("max_attempts", cfg.MaxAttempts).
			Str("next_delay", delay.String()).
			Msg("SSH connection attempt failed, retrying...")

		j.Deps.BroadcastServerEvent(server, "server.connection_attempt", map[string]any{
			"server_id":    server.ID,
			"attempt":      attempt,
			"max_attempts": cfg.MaxAttempts,
			"status":       "retrying",
		})
	}

	attempts, err := retry.DoWithAttempts(ctx, cfg, func() error {
		// Reload server to get latest data (including keys)
		server, err := j.Deps.Repos.Server().FindByID(ctx, j.Payload.ServerID)
		if err != nil {
			return fmt.Errorf("failed to reload server: %w", err)
		}

		// Skip if no private key yet
		if server.PrivateKey.IsEmpty() {
			j.Deps.Logger.Info().
				Str("server_id", server.ID).
				Msg("Server has no private key yet, waiting...")
			return fmt.Errorf("server has no private key")
		}

		// Skip if no IP yet
		if server.PublicIPv4 == nil || *server.PublicIPv4 == "" {
			j.Deps.Logger.Info().
				Str("server_id", server.ID).
				Msg("Server has no IP address yet, waiting...")
			return fmt.Errorf("server has no IP address")
		}

		// Try to connect using whoami task
		task := tasks.Whoami()

		connCtx, cancel := context.WithTimeout(ctx, connectionTimeout)
		result, connErr := j.Deps.RunTask(server, task).
			WithoutTracking().
			AsRoot().
			Dispatch(connCtx)
		cancel()

		if connErr != nil || result == nil || !result.IsSuccessful() {
			return fmt.Errorf("SSH connection failed")
		}

		j.Deps.Logger.Info().
			Str("server_id", server.ID).
			Str("output", result.GetOutput()).
			Msg("SSH connection successful")
		return nil
	})

	return err == nil, attempts
}

// dispatchProvisionJob dispatches the ProvisionServer job
func (j *WaitForServerToConnectJob) dispatchProvisionJob() error {
	task, err := NewProvisionServerTask(
		j.Payload.ServerID,
		j.Payload.TeamID,
		j.Payload.UserID,
		j.Payload.SSHKeyIDs,
	)
	if err != nil {
		return fmt.Errorf("failed to create provision task: %w", err)
	}

	if err := j.Deps.DispatchTask(task); err != nil {
		return fmt.Errorf("failed to dispatch provision job: %w", err)
	}

	j.Deps.Logger.Info().
		Str("server_id", j.Payload.ServerID).
		Msg("ProvisionServer job dispatched")

	return nil
}

func (j *WaitForServerToConnectJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).
		Str("server_id", j.Payload.ServerID).
		Msg("Failed to wait for server connection")

	// Update server status to failed
	if updateErr := j.Deps.Repos.Server().UpdateStatus(ctx, j.Payload.ServerID, types.ServerStatusFailed); updateErr != nil {
		j.Deps.Logger.Error().Err(updateErr).Msg("Failed to update server status to failed")
	}

	// Update connectivity status
	if updateErr := j.Deps.Repos.Server().UpdateFields(ctx, j.Payload.ServerID, map[string]any{
		"connected": false,
	}); updateErr != nil {
		j.Deps.Logger.Error().Err(updateErr).
			Str("server_id", j.Payload.ServerID).
			Msg("Failed to mark server disconnected after connection failure")
	}

	server, findErr := j.Deps.Repos.Server().FindByID(ctx, j.Payload.ServerID)
	if findErr == nil {
		j.Deps.BroadcastServerEvent(server, "server.connection_failed", map[string]any{
			"server_id": j.Payload.ServerID,
			"error":     err.Error(),
		})
	}

	// Dispatch CleanupFailedServerProvisioning job
	j.dispatchCleanupJob(err.Error())
}

// dispatchCleanupJob dispatches the cleanup job for failed provisioning
func (j *WaitForServerToConnectJob) dispatchCleanupJob(reason string) {
	if j.Deps.Queue == nil {
		j.Deps.Logger.Error().Msg("Queue not available, cannot dispatch cleanup job")
		return
	}

	task, err := NewCleanupFailedProvisioningTask(
		j.Payload.ServerID,
		j.Payload.TeamID,
		j.Payload.ServerProviderID,
		j.Payload.UserID,
		"Connection failed: "+reason,
		false, // Don't delete the record, keep it for debugging
	)
	if err != nil {
		j.Deps.Logger.Error().Err(err).Msg("Failed to create cleanup task")
		return
	}

	if err := j.Deps.DispatchTask(task); err != nil {
		return // Error already logged by DispatchTask
	}

	j.Deps.Logger.Info().
		Str("server_id", j.Payload.ServerID).
		Msg("CleanupFailedProvisioning job dispatched")
}

// NewWaitForServerToConnectTask creates an asynq task for waiting for server connection
// Uses TaskID for deduplication to prevent duplicate wait jobs
func NewWaitForServerToConnectTask(serverID, teamID string, serverProviderID string, userID *string, sshKeyIDs []string) (*asynq.Task, error) {
	return pkgjobs.TaskWithID(TypeWaitForServerToConnect,
		WaitForServerToConnectPayload{
			ServerID:         serverID,
			TeamID:           teamID,
			ServerProviderID: serverProviderID,
			UserID:           userID,
			SSHKeyIDs:        sshKeyIDs,
		},
		pkgjobs.Dedup("wait_connect", serverID),
	)
}
