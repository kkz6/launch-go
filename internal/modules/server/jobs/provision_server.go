package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/enums"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/pkg/jobs"
)

// ProvisionServerJob provisions a fresh server with all required software.
// Similar to Laravel's Modules\Server\Jobs\ProvisionServer
type ProvisionServerJob struct {
	ServerJobBase
	jobs.StatusTracker
	Payload ProvisionServerPayload
}

// Type returns the job type identifier
func (j *ProvisionServerJob) Type() string {
	return TypeProvisionServer
}

// Handle processes the job
func (j *ProvisionServerJob) Handle(ctx context.Context) error {
	// Find the server
	server, err := j.Repo().FindServerByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	// Update server status to provisioning
	if err := j.Repo().UpdateServerStatus(ctx, server.ID, enums.ServerStatusProvisioning); err != nil {
		return fmt.Errorf("failed to update server status: %w", err)
	}

	// Collect SSH keys if specified
	var sshKeyContents []string
	if len(j.Payload.SSHKeyIDs) > 0 {
		for _, keyID := range j.Payload.SSHKeyIDs {
			key, err := j.Repo().FindSshKeyByID(ctx, keyID)
			if err != nil {
				j.LogError(err, "Failed to find SSH key", "key_id", keyID)
				continue
			}
			sshKeyContents = append(sshKeyContents, key.PublicKey)
		}
	}

	// Build provision task configuration
	config := tasks.ProvisionFreshServerConfig{
		MemoryInMB:       getMemoryInMB(server),
		PublicIPv4:       getPublicIP(server),
		Provider:         string(server.Provider),
		PublicKey:        server.PublicKey.String(),
		Username:         server.GetUsername(),
		Password:         server.Password.String(),
		WorkingDirectory: getWorkingDir(server),
		SSHKeys:          sshKeyContents,
		SSHPort:          server.GetSSHPort(),
		SoftwareStack:    getDefaultSoftwareStack(),
		DatabasePassword: server.DatabasePassword.String(),
	}

	// Create and run the provision task
	task := tasks.ProvisionFreshServer(config)

	// Run the task with tracking in background
	taskModel, err := j.RunTaskOnServer(server, task).
		AsRoot().
		TrackInDB().
		RunInBackground(ctx)

	if err != nil {
		return fmt.Errorf("failed to execute provision task: %w", err)
	}

	j.LogInfo("Server provisioning started",
		"server_id", server.ID,
		"server_name", server.Name,
		"task_id", taskModel.ID,
	)

	// Broadcast server updated event
	j.BroadcastServerEvent(server.ID, "server.provisioning", map[string]interface{}{
		"server_id": server.ID,
		"status":    "provisioning",
		"task_id":   taskModel.ID,
	})

	return nil
}

// Failed is called when the job fails after all retries
func (j *ProvisionServerJob) Failed(ctx context.Context, err error) {
	j.LogError(err, "Failed to provision server",
		"server_id", j.Payload.ServerID,
	)

	// Update server status to failed
	if updateErr := j.Repo().UpdateServerStatus(ctx, j.Payload.ServerID, enums.ServerStatusFailed); updateErr != nil {
		j.LogError(updateErr, "Failed to update server status to failed")
	}

	// Broadcast failure
	j.BroadcastServerEvent(j.Payload.ServerID, "server.provision_failed", map[string]interface{}{
		"server_id": j.Payload.ServerID,
		"error":     err.Error(),
	})
}

// Helper functions

func getMemoryInMB(server *models.Server) int {
	if server.MemoryInMB != nil {
		return *server.MemoryInMB
	}
	return 1024 // Default 1GB
}

func getPublicIP(server *models.Server) string {
	if server.PublicIPv4 != nil {
		return *server.PublicIPv4
	}
	return ""
}

func getWorkingDir(server *models.Server) string {
	if server.WorkingDirectory != nil {
		return *server.WorkingDirectory
	}
	return ".launch"
}

func getDefaultSoftwareStack() []enums.Software {
	return []enums.Software{
		enums.SoftwareCaddy2,
		enums.SoftwarePhp83,
		enums.SoftwareComposer2,
		enums.SoftwareMySql80,
		enums.SoftwareRedis,
		enums.SoftwareSupervisor,
	}
}

// NewProvisionServerTask creates an asynq task for provisioning a server
func NewProvisionServerTask(serverID, teamID string, userID *string, sshKeyIDs []string) (*asynq.Task, error) {
	return jobs.NewTask(TypeProvisionServer, ProvisionServerPayload{
		ServerID:  serverID,
		TeamID:    teamID,
		UserID:    userID,
		SSHKeyIDs: sshKeyIDs,
	})
}
