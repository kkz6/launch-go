package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/modules/server/types"
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
	Deps    *JobDeps
	Payload ProvisionServerPayload

	server *models.Server
}

func NewProvisionServerJob(p ProvisionServerPayload) pkgjobs.Handler {
	return &ProvisionServerJob{Deps: deps, Payload: p}
}

func (j *ProvisionServerJob) Handle(ctx context.Context) error {
	var err error
	j.server, err = j.Deps.Repos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	if err := j.Deps.Repos.Server().UpdateStatus(ctx, j.server.ID, types.ServerStatusProvisioning); err != nil {
		return fmt.Errorf("failed to update server status: %w", err)
	}

	var sshKeyContents []string
	if len(j.Payload.SSHKeyIDs) > 0 {
		for _, keyID := range j.Payload.SSHKeyIDs {
			key, err := j.Deps.Repos.SSHKey().FindByID(ctx, keyID)
			if err != nil {
				j.Deps.Logger.Error().Err(err).
					Str("key_id", keyID).
					Msg("failed to find SSH key")
				continue
			}
			sshKeyContents = append(sshKeyContents, key.PublicKey)
		}
	}

	// Generate signed URL for launch-agent pulse webhook
	agentURL := generateAgentPulseURL(j.server.ID)

	config := tasks.ProvisionFreshServerConfig{
		ServerID:         j.server.ID,
		TeamID:           j.server.TeamID,
		MemoryInMB:       getMemoryInMB(j.server),
		PublicIPv4:       getPublicIP(j.server),
		Provider:         string(j.server.Provider),
		PublicKey:        j.server.PublicKey.String(),
		Username:         j.server.GetUsername(),
		Password:         j.server.Password.String(),
		WorkingDirectory: getWorkingDir(j.server),
		SSHKeys:          sshKeyContents,
		SSHPort:          j.server.GetSSHPort(),
		SoftwareStack:    getDefaultSoftwareStack(),
		DatabasePassword: j.server.DatabasePassword.String(),
		AgentConfigPath:  "/etc/launch-agent/launch-agent.yaml",
		AgentURL:         agentURL,
	}

	task := tasks.ProvisionFreshServer(config)

	taskModel, err := j.Deps.RunTask(j.server, task).
		AsRoot().
		TrackInDB().
		RunInBackground(ctx)

	if err != nil {
		return fmt.Errorf("failed to execute provision task: %w", err)
	}

	j.Deps.Logger.Info().
		Str("server_id", j.server.ID).
		Str("server_name", j.server.Name).
		Str("task_id", taskModel.ID).
		Msg("server provisioning started")

	j.Deps.BroadcastServerEvent(j.server, "server.provisioning", map[string]any{
		"server_id": j.server.ID,
		"status":    "provisioning",
		"task_id":   taskModel.ID,
	})

	return nil
}

func (j *ProvisionServerJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).
		Str("server_id", j.Payload.ServerID).
		Msg("failed to provision server")

	if updateErr := j.Deps.Repos.Server().UpdateStatus(ctx, j.Payload.ServerID, types.ServerStatusFailed); updateErr != nil {
		j.Deps.Logger.Error().Err(updateErr).
			Msg("failed to update server status to failed")
	}

	server, findErr := j.Deps.Repos.Server().FindByID(ctx, j.Payload.ServerID)
	if findErr == nil {
		j.Deps.BroadcastServerEvent(server, "server.provision_failed", map[string]any{
			"server_id": j.Payload.ServerID,
			"error":     err.Error(),
		})

		// Dispatch CleanupFailedServerProvisioning job
		j.dispatchCleanupJob(server, err.Error())
	}
}

// dispatchCleanupJob dispatches the cleanup job for failed provisioning
func (j *ProvisionServerJob) dispatchCleanupJob(server *models.Server, reason string) {
	if j.Deps.Queue == nil {
		j.Deps.Logger.Error().
			Msg("queue not available, cannot dispatch cleanup job")
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
		j.Deps.Logger.Error().Err(err).
			Msg("failed to create cleanup task")
		return
	}

	if err := j.Deps.DispatchTask(task); err != nil {
		return // Error already logged by DispatchTask
	}

	j.Deps.Logger.Info().
		Str("server_id", j.Payload.ServerID).
		Msg("CleanupFailedProvisioning job dispatched")
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

func getDefaultSoftwareStack() []types.Software {
	return []types.Software{
		types.SoftwareCaddy2,
		types.SoftwarePhp83,
		types.SoftwareComposer2,
		types.SoftwareMySQL80,
		types.SoftwareRedis,
		types.SoftwareSupervisor,
	}
}

// NewProvisionServerTask creates an asynq task for provisioning a server.
// Uses TaskID for deduplication to prevent duplicate provisioning
func NewProvisionServerTask(serverID, teamID string, userID *string, sshKeyIDs []string) (*asynq.Task, error) {
	return pkgjobs.TaskWithID(TypeProvisionServer,
		ProvisionServerPayload{
			ServerID:  serverID,
			TeamID:    teamID,
			UserID:    userID,
			SSHKeyIDs: sshKeyIDs,
		},
		pkgjobs.Dedup("provision", serverID),
	)
}

// generateAgentPulseURL creates a permanent signed URL for the launch-agent pulse webhook
func generateAgentPulseURL(serverID string) string {
	path := fmt.Sprintf("/webhooks/servers/%s/pulse", serverID)
	return signedurl.PermanentSign(path, nil)
}
