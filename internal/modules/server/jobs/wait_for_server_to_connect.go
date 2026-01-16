package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/enums"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeWaitForServerToConnect = "server:wait_for_connect"

// Connection retry configuration
const (
	maxConnectionAttempts = 30          // Maximum number of connection attempts
	initialRetryDelay     = 10 * time.Second
	maxRetryDelay         = 30 * time.Second
	connectionTimeout     = 15 * time.Second
)

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
	ctx     *JobContext
	Payload WaitForServerToConnectPayload
}

func (j *WaitForServerToConnectJob) Handle(ctx context.Context) error {
	server, err := j.ctx.Repos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	// Update status to indicate we're waiting for connection
	if err := j.ctx.Repos.Server().UpdateStatus(ctx, server.ID, enums.ServerStatusStarting); err != nil {
		return fmt.Errorf("failed to update server status: %w", err)
	}

	j.ctx.BroadcastServerEvent(server, "server.waiting_for_connection", map[string]any{
		"server_id": server.ID,
		"status":    "waiting_for_connection",
	})

	j.ctx.LogInfo("Waiting for server to become accessible",
		"server_id", server.ID,
		"server_name", server.Name,
	)

	// Wait for server to have an IP address
	if err = j.waitForIP(ctx); err != nil {
		return fmt.Errorf("failed to get server IP: %w", err)
	}

	// Reload server with IP
	server, err = j.ctx.Repos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to reload server: %w", err)
	}

	// Attempt to connect with retry logic
	connected, attempt := j.attemptConnection(ctx)
	if !connected {
		return fmt.Errorf("server failed to become accessible after %d attempts", attempt)
	}

	// Update connectivity status
	if err := j.ctx.Repos.Server().UpdateFields(ctx, server.ID, map[string]any{
		"is_connected": true,
	}); err != nil {
		j.ctx.LogError(err, "Failed to update connectivity status")
	}

	j.ctx.LogInfo("Server is now accessible, dispatching provisioning job",
		"server_id", server.ID,
		"attempts", attempt,
	)

	j.ctx.BroadcastServerEvent(server, "server.connected", map[string]any{
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
	maxIPAttempts := 30
	for attempt := 1; attempt <= maxIPAttempts; attempt++ {
		server, err := j.ctx.Repos.Server().FindByID(ctx, j.Payload.ServerID)
		if err != nil {
			return err
		}

		if server.PublicIPv4 != nil && *server.PublicIPv4 != "" {
			return nil
		}

		j.ctx.LogInfo("Waiting for server IP address",
			"server_id", server.ID,
			"attempt", attempt,
		)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Second):
		}
	}

	return fmt.Errorf("timeout waiting for server IP address")
}

// attemptConnection tries to connect to the server with exponential backoff
func (j *WaitForServerToConnectJob) attemptConnection(ctx context.Context) (bool, int) {
	delay := initialRetryDelay

	for attempt := 1; attempt <= maxConnectionAttempts; attempt++ {
		// Reload server to get latest data (including keys)
		server, err := j.ctx.Repos.Server().FindByID(ctx, j.Payload.ServerID)
		if err != nil {
			j.ctx.LogError(err, "Failed to reload server",
				"attempt", attempt,
			)
			continue
		}

		// Skip if no private key yet
		if server.PrivateKey.IsEmpty() {
			j.ctx.LogInfo("Server has no private key yet, waiting...",
				"server_id", server.ID,
				"attempt", attempt,
			)
			time.Sleep(delay)
			continue
		}

		// Skip if no IP yet
		if server.PublicIPv4 == nil || *server.PublicIPv4 == "" {
			j.ctx.LogInfo("Server has no IP address yet, waiting...",
				"server_id", server.ID,
				"attempt", attempt,
			)
			time.Sleep(delay)
			continue
		}

		// Try to connect using whoami task
		task := tasks.Whoami()

		connCtx, cancel := context.WithTimeout(ctx, connectionTimeout)
		result, err := j.ctx.ForServer(server).RunTask(task).
			AsRoot().
			Dispatch(connCtx)
		cancel()

		if err == nil && result != nil && result.IsSuccessful() {
			j.ctx.LogInfo("SSH connection successful",
				"server_id", server.ID,
				"attempt", attempt,
				"output", result.GetOutput(),
			)
			return true, attempt
		}

		j.ctx.LogInfo("SSH connection attempt failed, retrying...",
			"server_id", server.ID,
			"attempt", attempt,
			"max_attempts", maxConnectionAttempts,
			"next_delay", delay.String(),
		)

		j.ctx.BroadcastServerEvent(server, "server.connection_attempt", map[string]any{
			"server_id":    server.ID,
			"attempt":      attempt,
			"max_attempts": maxConnectionAttempts,
			"status":       "retrying",
		})

		select {
		case <-ctx.Done():
			return false, attempt
		case <-time.After(delay):
		}

		// Increase delay with exponential backoff, capped at maxRetryDelay
		delay = time.Duration(float64(delay) * 1.2)
		if delay > maxRetryDelay {
			delay = maxRetryDelay
		}
	}

	return false, maxConnectionAttempts
}

// dispatchProvisionJob dispatches the ProvisionServer job
func (j *WaitForServerToConnectJob) dispatchProvisionJob() error {
	if j.ctx.Queue == nil {
		return fmt.Errorf("queue client not available")
	}

	task, err := NewProvisionServerTask(
		j.Payload.ServerID,
		j.Payload.TeamID,
		j.Payload.UserID,
		j.Payload.SSHKeyIDs,
	)
	if err != nil {
		return fmt.Errorf("failed to create provision task: %w", err)
	}

	if _, err := j.ctx.Queue.Enqueue(task); err != nil {
		return fmt.Errorf("failed to enqueue provision job: %w", err)
	}

	j.ctx.LogInfo("ProvisionServer job dispatched",
		"server_id", j.Payload.ServerID,
	)

	return nil
}

func (j *WaitForServerToConnectJob) Failed(ctx context.Context, err error) {
	j.ctx.LogError(err, "Failed to wait for server connection",
		"server_id", j.Payload.ServerID,
	)

	// Update server status to failed
	if updateErr := j.ctx.Repos.Server().UpdateStatus(ctx, j.Payload.ServerID, enums.ServerStatusFailed); updateErr != nil {
		j.ctx.LogError(updateErr, "Failed to update server status to failed")
	}

	// Update connectivity status
	_ = j.ctx.Repos.Server().UpdateFields(ctx, j.Payload.ServerID, map[string]any{
		"is_connected": false,
	})

	server, findErr := j.ctx.Repos.Server().FindByID(ctx, j.Payload.ServerID)
	if findErr == nil {
		j.ctx.BroadcastServerEvent(server, "server.connection_failed", map[string]any{
			"server_id": j.Payload.ServerID,
			"error":     err.Error(),
		})
	}

	// Dispatch CleanupFailedServerProvisioning job
	j.dispatchCleanupJob(err.Error())
}

// dispatchCleanupJob dispatches the cleanup job for failed provisioning
func (j *WaitForServerToConnectJob) dispatchCleanupJob(reason string) {
	if j.ctx.Queue == nil {
		j.ctx.LogError(nil, "Queue not available, cannot dispatch cleanup job")
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
		j.ctx.LogError(err, "Failed to create cleanup task")
		return
	}

	if _, err := j.ctx.Queue.Enqueue(task); err != nil {
		j.ctx.LogError(err, "Failed to enqueue cleanup job")
		return
	}

	j.ctx.LogInfo("CleanupFailedProvisioning job dispatched",
		"server_id", j.Payload.ServerID,
	)
}

// NewWaitForServerToConnectJob creates a new WaitForServerToConnectJob
func NewWaitForServerToConnectJob(ctx *JobContext, payload WaitForServerToConnectPayload) *WaitForServerToConnectJob {
	return &WaitForServerToConnectJob{
		ctx:     ctx,
		Payload: payload,
	}
}

// NewWaitForServerToConnectTask creates an asynq task for waiting for server connection
func NewWaitForServerToConnectTask(serverID, teamID string, serverProviderID string, userID *string, sshKeyIDs []string) (*asynq.Task, error) {
	return pkgjobs.NewTask(TypeWaitForServerToConnect, WaitForServerToConnectPayload{
		ServerID:         serverID,
		TeamID:           teamID,
		ServerProviderID: serverProviderID,
		UserID:           userID,
		SSHKeyIDs:        sshKeyIDs,
	})
}
