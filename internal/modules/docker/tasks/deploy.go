package tasks

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/kkz6/launch-go/internal/modules/docker/models"
	dockertypes "github.com/kkz6/launch-go/internal/modules/docker/types"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner/templates"
)

const DeployDockerServiceTaskType = "docker:deploy"

type DeployConfig struct {
	ContainerName    string
	Image            string
	RestartPolicy    string
	RegistryURL      string
	RegistryUsername string
	RegistryPassword string
	EnvVars          []EnvVar
	Volumes          []Volume
	Ports            []Port
	CPULimit         string
	MemoryLimit      string
	Command          string
}

type EnvVar struct {
	Key   string
	Value string
}

type Volume struct {
	Source   string
	Target   string
	ReadOnly bool
}

type Port struct {
	HostPort      int
	ContainerPort int
	Protocol      string
}

type deployCallbackData struct {
	DockerServiceID string `json:"docker_service_id"`
	DeploymentID    string `json:"deployment_id"`
	TeamID          string `json:"team_id"`
	ServerID        string `json:"server_id"`
}

type DeployDockerServiceTask struct {
	*taskrunner.BaseTask
	callback deployCallbackData
}

func DeployDockerService(config DeployConfig, serviceID, deploymentID, teamID, serverID string) *DeployDockerServiceTask {
	script := templates.MustRender("docker", "docker/deploy.sh", config)

	return &DeployDockerServiceTask{
		BaseTask: taskrunner.NewBaseTask(
			taskrunner.WithName("Deploy Docker Service"),
			taskrunner.WithScript(script),
			taskrunner.WithTimeoutSeconds(600),
		),
		callback: deployCallbackData{
			DockerServiceID: serviceID,
			DeploymentID:    deploymentID,
			TeamID:          teamID,
			ServerID:        serverID,
		},
	}
}

func (t *DeployDockerServiceTask) TypeName() string {
	return DeployDockerServiceTaskType
}

func (t *DeployDockerServiceTask) MarshalPayload() ([]byte, error) {
	return json.Marshal(t.callback)
}

func (t *DeployDockerServiceTask) OnSuccess(ctx context.Context, cbCtx *taskrunner.CallbackContext, taskID string) error {
	now := time.Now()

	if cbCtx.Logger != nil {
		cbCtx.Logger.Info().
			Str("task_id", taskID).
			Str("docker_service_id", t.callback.DockerServiceID).
			Str("deployment_id", t.callback.DeploymentID).
			Msg("DeployDockerService: onSuccess callback triggered")
	}

	if err := cbCtx.DB.Model(&models.DockerDeployment{}).
		Where("id = ?", t.callback.DeploymentID).
		Updates(map[string]interface{}{
			"status":      dockertypes.DeploymentStatusFinished,
			"finished_at": now,
		}).Error; err != nil {
		return fmt.Errorf("failed to update deployment status: %w", err)
	}

	if err := cbCtx.DB.Model(&models.DockerService{}).
		Where("id = ?", t.callback.DockerServiceID).
		Updates(map[string]interface{}{
			"status":       dockertypes.ServiceStatusRunning,
			"installed_at": now,
		}).Error; err != nil {
		return fmt.Errorf("failed to update service status: %w", err)
	}

	cbCtx.BroadcastToTeam(t.callback.TeamID, "docker_service.deployed", map[string]interface{}{
		"docker_service_id": t.callback.DockerServiceID,
		"deployment_id":     t.callback.DeploymentID,
		"status":            "running",
	})

	return nil
}

func (t *DeployDockerServiceTask) OnFailure(ctx context.Context, cbCtx *taskrunner.CallbackContext, taskID string, exitCode int) error {
	now := time.Now()

	if cbCtx.Logger != nil {
		cbCtx.Logger.Error().
			Str("task_id", taskID).
			Str("docker_service_id", t.callback.DockerServiceID).
			Str("deployment_id", t.callback.DeploymentID).
			Int("exit_code", exitCode).
			Msg("DeployDockerService: onFailure callback triggered")
	}

	if err := cbCtx.DB.Model(&models.DockerDeployment{}).
		Where("id = ?", t.callback.DeploymentID).
		Updates(map[string]interface{}{
			"status":      dockertypes.DeploymentStatusFailed,
			"finished_at": now,
		}).Error; err != nil {
		return fmt.Errorf("failed to update deployment status: %w", err)
	}

	if err := cbCtx.DB.Model(&models.DockerService{}).
		Where("id = ?", t.callback.DockerServiceID).
		Update("status", dockertypes.ServiceStatusFailed).Error; err != nil {
		return fmt.Errorf("failed to update service status: %w", err)
	}

	cbCtx.BroadcastToTeam(t.callback.TeamID, "docker_service.deploy_failed", map[string]interface{}{
		"docker_service_id": t.callback.DockerServiceID,
		"deployment_id":     t.callback.DeploymentID,
		"exit_code":         exitCode,
	})

	return nil
}

func (t *DeployDockerServiceTask) OnExpired(ctx context.Context, cbCtx *taskrunner.CallbackContext, taskID string) error {
	if cbCtx.Logger != nil {
		cbCtx.Logger.Error().
			Str("task_id", taskID).
			Str("docker_service_id", t.callback.DockerServiceID).
			Str("deployment_id", t.callback.DeploymentID).
			Msg("DeployDockerService: onExpired callback triggered")
	}

	return t.OnFailure(ctx, cbCtx, taskID, -1)
}

func (s deployCallbackData) NewTask() taskrunner.CallbackHandler {
	return &DeployDockerServiceTask{
		BaseTask: taskrunner.NewBaseTask(),
		callback: s,
	}
}
