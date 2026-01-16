package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/enums"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeProvisionServer = "server:provision"

type ProvisionServerPayload struct {
	ServerID  string   `json:"server_id"`
	TeamID    string   `json:"team_id"`
	UserID    *string  `json:"user_id,omitempty"`
	SSHKeyIDs []string `json:"ssh_key_ids,omitempty"`
}

type ProvisionServerJob struct {
	ctx     *JobContext
	Payload ProvisionServerPayload
}

func (j *ProvisionServerJob) Handle(ctx context.Context) error {
	server, err := j.ctx.Repo.FindServerByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	if err := j.ctx.Repo.UpdateServerStatus(ctx, server.ID, enums.ServerStatusProvisioning); err != nil {
		return fmt.Errorf("failed to update server status: %w", err)
	}

	var sshKeyContents []string
	if len(j.Payload.SSHKeyIDs) > 0 {
		for _, keyID := range j.Payload.SSHKeyIDs {
			key, err := j.ctx.Repo.FindSshKeyByID(ctx, keyID)
			if err != nil {
				j.ctx.LogError(err, "Failed to find SSH key", "key_id", keyID)
				continue
			}
			sshKeyContents = append(sshKeyContents, key.PublicKey)
		}
	}

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

	task := tasks.ProvisionFreshServer(config)

	taskModel, err := j.ctx.ForServer(server).RunTask(task).
		AsRoot().
		TrackInDB().
		RunInBackground(ctx)

	if err != nil {
		return fmt.Errorf("failed to execute provision task: %w", err)
	}

	j.ctx.LogInfo("Server provisioning started",
		"server_id", server.ID,
		"server_name", server.Name,
		"task_id", taskModel.ID,
	)

	j.ctx.BroadcastToServer(server.ID, "server.provisioning", map[string]any{
		"server_id": server.ID,
		"status":    "provisioning",
		"task_id":   taskModel.ID,
	})

	return nil
}

func (j *ProvisionServerJob) Failed(ctx context.Context, err error) {
	j.ctx.LogError(err, "Failed to provision server",
		"server_id", j.Payload.ServerID,
	)

	if updateErr := j.ctx.Repo.UpdateServerStatus(ctx, j.Payload.ServerID, enums.ServerStatusFailed); updateErr != nil {
		j.ctx.LogError(updateErr, "Failed to update server status to failed")
	}

	j.ctx.BroadcastToServer(j.Payload.ServerID, "server.provision_failed", map[string]any{
		"server_id": j.Payload.ServerID,
		"error":     err.Error(),
	})
}

// NewProvisionServerJob creates a new ProvisionServerJob with the given context and payload.
func NewProvisionServerJob(ctx *JobContext, payload ProvisionServerPayload) *ProvisionServerJob {
	return &ProvisionServerJob{
		ctx:     ctx,
		Payload: payload,
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
func NewProvisionServerTask(serverID, teamID string, userID *string, sshKeyIDs []string) (*asynq.Task, error) {
	return pkgjobs.NewTask(TypeProvisionServer, ProvisionServerPayload{
		ServerID:  serverID,
		TeamID:    teamID,
		UserID:    userID,
		SSHKeyIDs: sshKeyIDs,
	})
}
