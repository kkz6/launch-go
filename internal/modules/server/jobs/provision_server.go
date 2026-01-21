package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/enums"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	"github.com/kkz6/launch-go/internal/pkg/signedurl"
)

const TypeProvisionServer = "server:provision"

type ProvisionServerPayload struct {
	ServerID  string   `json:"server_id"`
	TeamID    string   `json:"team_id"`
	UserID    *string  `json:"user_id,omitempty"`
	SSHKeyIDs []string `json:"ssh_key_ids,omitempty"`
}

type ProvisionServerJob struct {
	pkgjobs.BaseJob[*JobContext, ProvisionServerPayload]
}

func (j *ProvisionServerJob) Handle(ctx context.Context) error {
	server, err := j.Ctx.Repos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	if err := j.Ctx.Repos.Server().UpdateStatus(ctx, server.ID, enums.ServerStatusProvisioning); err != nil {
		return fmt.Errorf("failed to update server status: %w", err)
	}

	var sshKeyContents []string
	if len(j.Payload.SSHKeyIDs) > 0 {
		for _, keyID := range j.Payload.SSHKeyIDs {
			key, err := j.Ctx.Repos.SSHKey().FindByID(ctx, keyID)
			if err != nil {
				j.Ctx.LogError(err, "Failed to find SSH key", "key_id", keyID)
				continue
			}
			sshKeyContents = append(sshKeyContents, key.PublicKey)
		}
	}

	// Generate signed URL for launch-agent pulse webhook
	agentURL := generateAgentPulseURL(server.ID)

	config := tasks.ProvisionFreshServerConfig{
		ServerID:         server.ID,
		TeamID:           server.TeamID,
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
		AgentConfigPath:  "/etc/launch-agent/launch-agent.yaml",
		AgentURL:         agentURL,
	}

	task := tasks.ProvisionFreshServer(config)

	taskModel, err := j.Ctx.ForServer(server).RunTask(task).
		AsRoot().
		TrackInDB().
		RunInBackground(ctx)

	if err != nil {
		return fmt.Errorf("failed to execute provision task: %w", err)
	}

	j.Ctx.LogInfo("Server provisioning started",
		"server_id", server.ID,
		"server_name", server.Name,
		"task_id", taskModel.ID,
	)

	j.Ctx.BroadcastServerEvent(server, "server.provisioning", map[string]any{
		"server_id": server.ID,
		"status":    "provisioning",
		"task_id":   taskModel.ID,
	})

	return nil
}

func (j *ProvisionServerJob) Failed(ctx context.Context, err error) {
	j.Ctx.LogError(err, "Failed to provision server",
		"server_id", j.Payload.ServerID,
	)

	if updateErr := j.Ctx.Repos.Server().UpdateStatus(ctx, j.Payload.ServerID, enums.ServerStatusFailed); updateErr != nil {
		j.Ctx.LogError(updateErr, "Failed to update server status to failed")
	}

	server, findErr := j.Ctx.Repos.Server().FindByID(ctx, j.Payload.ServerID)
	if findErr == nil {
		j.Ctx.BroadcastServerEvent(server, "server.provision_failed", map[string]any{
			"server_id": j.Payload.ServerID,
			"error":     err.Error(),
		})

		// Dispatch CleanupFailedServerProvisioning job
		j.dispatchCleanupJob(server, err.Error())
	}
}

// dispatchCleanupJob dispatches the cleanup job for failed provisioning
func (j *ProvisionServerJob) dispatchCleanupJob(server *models.Server, reason string) {
	if j.Ctx.Queue == nil {
		j.Ctx.LogError(nil, "Queue not available, cannot dispatch cleanup job")
		return
	}

	serverProviderID := ""
	if server.ServerProviderID != nil {
		serverProviderID = *server.ServerProviderID
	}

	task, err := NewCleanupFailedProvisioningTask(
		j.Payload.ServerID,
		j.Payload.TeamID,
		serverProviderID,
		j.Payload.UserID,
		"Provisioning failed: "+reason,
		false, // Don't delete the record, keep it for debugging
	)
	if err != nil {
		j.Ctx.LogError(err, "Failed to create cleanup task")
		return
	}

	if _, err := j.Ctx.Queue.Enqueue(task); err != nil {
		j.Ctx.LogError(err, "Failed to enqueue cleanup job")
		return
	}

	j.Ctx.LogInfo("CleanupFailedProvisioning job dispatched",
		"server_id", j.Payload.ServerID,
	)
}

// NewProvisionServerJob creates a new ProvisionServerJob with the given context and payload.
func NewProvisionServerJob(ctx *JobContext, payload ProvisionServerPayload) *ProvisionServerJob {
	return &ProvisionServerJob{
		BaseJob: pkgjobs.NewBaseJob(ctx, payload),
	}
}

func getMemoryInMB(server *models.Server) int {
	if server.MemoryInMB != nil {
		return *server.MemoryInMB
	}
	return 1024
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

// NewProvisionServerTask creates an asynq task for provisioning a server.
// Uses TaskID for deduplication to prevent duplicate provisioning
func NewProvisionServerTask(serverID, teamID string, userID *string, sshKeyIDs []string) (*asynq.Task, error) {
	return pkgjobs.NewTask(TypeProvisionServer, ProvisionServerPayload{
		ServerID:  serverID,
		TeamID:    teamID,
		UserID:    userID,
		SSHKeyIDs: sshKeyIDs,
	}, asynq.TaskID(fmt.Sprintf("provision:%s", serverID)))
}

// generateAgentPulseURL creates a permanent signed URL for the launch-agent pulse webhook
func generateAgentPulseURL(serverID string) string {
	path := fmt.Sprintf("/webhooks/servers/%s/pulse", serverID)
	return signedurl.PermanentSign(path, nil)
}
