package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/enums"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
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
	pkgjobs.BaseJob[*JobContext, WaitForServerToConnectPayload]
}

func (j *WaitForServerToConnectJob) Handle(ctx context.Context) error {
	server, err := j.Ctx.Repos().Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	// Update status to indicate we're waiting for connection
	if err := j.Ctx.Repos().Server().UpdateStatus(ctx, server.ID, enums.ServerStatusStarting); err != nil {
		return fmt.Errorf("failed to update server status: %w", err)
	}

	j.Ctx.BroadcastServerEvent(server, "server.waiting_for_connection", map[string]any{
		"server_id": server.ID,
		"status":    "waiting_for_connection",
	})

	j.Ctx.LogInfo("Waiting for server to become accessible",
		"server_id", server.ID,
		"server_name", server.Name,
	)

	// Wait for server to have an IP address
	if err = j.waitForIP(ctx); err != nil {
		return fmt.Errorf("failed to get server IP: %w", err)
	}

	// Reload server with IP
	server, err = j.Ctx.Repos().Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to reload server: %w", err)
	}

	// Attempt to connect with retry logic
	connected, attempt := j.attemptConnection(ctx)
	if !connected {
		return fmt.Errorf("server failed to become accessible after %d attempts", attempt)
	}

	// Update connectivity status
	if err := j.Ctx.Repos().Server().UpdateFields(ctx, server.ID, map[string]any{
		"is_connected": true,
	}); err != nil {
		j.Ctx.LogError(err, "Failed to update connectivity status")
	}

	j.Ctx.LogInfo("Server is now accessible, dispatching provisioning job",
		"server_id", server.ID,
		"attempts", attempt,
	)

	j.Ctx.BroadcastServerEvent(server, "server.connected", map[string]any{
		"server_id": server.ID,
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
		j.Ctx.LogInfo("Waiting for server IP address",
			"server_id", j.Payload.ServerID,
			"attempt", attempt,
		)
	}

	return retry.Do(ctx, cfg, func() error {
		server, err := j.Ctx.Repos().Server().FindByID(ctx, j.Payload.ServerID)
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
		server, err := j.Ctx.Repos().Server().FindByID(ctx, j.Payload.ServerID)
		if err != nil {
			return
		}

		j.Ctx.LogInfo("SSH connection attempt failed, retrying...",
			"server_id", server.ID,
			"attempt", attempt,
			"max_attempts", cfg.MaxAttempts,
			"next_delay", delay.String(),
		)

		j.Ctx.BroadcastServerEvent(server, "server.connection_attempt", map[string]any{
			"server_id":    server.ID,
			"attempt":      attempt,
			"max_attempts": cfg.MaxAttempts,
			"status":       "retrying",
		})
	}

	attempts, err := retry.DoWithAttempts(ctx, cfg, func() error {
		// Reload server to get latest data (including keys)
		server, err := j.Ctx.Repos().Server().FindByID(ctx, j.Payload.ServerID)
		if err != nil {
			return fmt.Errorf("failed to reload server: %w", err)
		}

		// Skip if no private key yet
		if server.PrivateKey.IsEmpty() {
			j.Ctx.LogInfo("Server has no private key yet, waiting...",
				"server_id", server.ID,
			)
			return fmt.Errorf("server has no private key")
		}

		// Skip if no IP yet
		if server.PublicIPv4 == nil || *server.PublicIPv4 == "" {
			j.Ctx.LogInfo("Server has no IP address yet, waiting...",
				"server_id", server.ID,
			)
			return fmt.Errorf("server has no IP address")
		}

		// Try to connect using whoami task
		task := tasks.Whoami()

		connCtx, cancel := context.WithTimeout(ctx, connectionTimeout)
		result, connErr := j.Ctx.ForServer(server).RunTask(task).
			AsRoot().
			Dispatch(connCtx)
		cancel()

		if connErr != nil || result == nil || !result.IsSuccessful() {
			return fmt.Errorf("SSH connection failed")
		}

		j.Ctx.LogInfo("SSH connection successful",
			"server_id", server.ID,
			"output", result.GetOutput(),
		)
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

	if err := j.Ctx.DispatchTask(task); err != nil {
		return fmt.Errorf("failed to dispatch provision job: %w", err)
	}

	j.Ctx.LogInfo("ProvisionServer job dispatched",
		"server_id", j.Payload.ServerID,
	)

	return nil
}

func (j *WaitForServerToConnectJob) Failed(ctx context.Context, err error) {
	j.Ctx.LogError(err, "Failed to wait for server connection",
		"server_id", j.Payload.ServerID,
	)

	// Update server status to failed
	if updateErr := j.Ctx.Repos().Server().UpdateStatus(ctx, j.Payload.ServerID, enums.ServerStatusFailed); updateErr != nil {
		j.Ctx.LogError(updateErr, "Failed to update server status to failed")
	}

	// Update connectivity status
	_ = j.Ctx.Repos().Server().UpdateFields(ctx, j.Payload.ServerID, map[string]any{
		"is_connected": false,
	})

	server, findErr := j.Ctx.Repos().Server().FindByID(ctx, j.Payload.ServerID)
	if findErr == nil {
		j.Ctx.BroadcastServerEvent(server, "server.connection_failed", map[string]any{
			"server_id": j.Payload.ServerID,
			"error":     err.Error(),
		})
	}

	// Dispatch CleanupFailedServerProvisioning job
	j.dispatchCleanupJob(err.Error())
}

// dispatchCleanupJob dispatches the cleanup job for failed provisioning
func (j *WaitForServerToConnectJob) dispatchCleanupJob(reason string) {
	if j.Ctx.Queue() == nil {
		j.Ctx.LogError(nil, "Queue not available, cannot dispatch cleanup job")
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
		j.Ctx.LogError(err, "Failed to create cleanup task")
		return
	}

	if err := j.Ctx.DispatchTask(task); err != nil {
		return // Error already logged by DispatchTask
	}

	j.Ctx.LogInfo("CleanupFailedProvisioning job dispatched",
		"server_id", j.Payload.ServerID,
	)
}

// NewWaitForServerToConnectJob creates a new WaitForServerToConnectJob
func NewWaitForServerToConnectJob(ctx *JobContext, payload WaitForServerToConnectPayload) *WaitForServerToConnectJob {
	return &WaitForServerToConnectJob{
		BaseJob: pkgjobs.NewBaseJob(ctx, payload),
	}
}

// NewWaitForServerToConnectTask creates an asynq task for waiting for server connection
// Uses TaskID for deduplication to prevent duplicate wait jobs
func NewWaitForServerToConnectTask(serverID, teamID string, serverProviderID string, userID *string, sshKeyIDs []string) (*asynq.Task, error) {
	return pkgjobs.NewTask(TypeWaitForServerToConnect, WaitForServerToConnectPayload{
		ServerID:         serverID,
		TeamID:           teamID,
		ServerProviderID: serverProviderID,
		UserID:           userID,
		SSHKeyIDs:        sshKeyIDs,
	}, asynq.TaskID(fmt.Sprintf("wait_connect:%s", serverID)))
}
