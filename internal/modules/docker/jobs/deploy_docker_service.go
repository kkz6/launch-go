package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/kkz6/launch-go/internal/modules/docker/models"
	"github.com/kkz6/launch-go/internal/modules/docker/tasks"
	dockertypes "github.com/kkz6/launch-go/internal/modules/docker/types"
	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeDeployDockerService = "docker:deploy_service"

type DeployDockerServicePayload struct {
	DockerServiceID string `json:"docker_service_id"`
	TeamID          string `json:"team_id"`
	ServerID        string `json:"server_id"`
}

type DeployDockerServiceJob struct {
	Deps    *JobDeps
	Payload DeployDockerServicePayload

	server  *servermodels.Server
	service *models.DockerService
}

func NewDeployDockerServiceJob(p DeployDockerServicePayload) pkgjobs.Handler {
	return &DeployDockerServiceJob{Deps: deps, Payload: p}
}

func (j *DeployDockerServiceJob) Handle(ctx context.Context) error {
	var err error

	j.service, err = j.Deps.Repos.DockerService().FindByID(ctx, j.Payload.DockerServiceID)
	if err != nil {
		return fmt.Errorf("failed to find docker service: %w", err)
	}

	j.server, err = j.loadServer(ctx)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	// Update service status to deploying
	if err := j.Deps.DB.Model(&models.DockerService{}).
		Where("id = ?", j.service.ID).
		Update("status", dockertypes.ServiceStatusDeploying).Error; err != nil {
		return fmt.Errorf("failed to update service status: %w", err)
	}

	// Create deployment record
	now := time.Now()
	deployment := &models.DockerDeployment{
		DockerServiceID: j.service.ID,
		Image:           j.service.Image,
		Status:          dockertypes.DeploymentStatusRunning,
		Trigger:         dockertypes.DeploymentTriggerManual,
		StartedAt:       &now,
	}

	if err := j.Deps.DB.Create(deployment).Error; err != nil {
		return fmt.Errorf("failed to create deployment record: %w", err)
	}

	// Build deploy config from service and its relations
	config := j.buildDeployConfig()

	task := tasks.DeployDockerService(config, j.service.ID, deployment.ID, j.Payload.TeamID, j.Payload.ServerID)

	taskModel, err := j.Deps.TaskRunnerDeps.NewRunner(j.server, task).
		AsRoot().
		TrackInDB().
		RunInBackground(ctx)

	if err != nil {
		return fmt.Errorf("failed to execute deploy task: %w", err)
	}

	j.Deps.Logger.Info().
		Str("docker_service_id", j.service.ID).
		Str("deployment_id", deployment.ID).
		Str("task_id", taskModel.ID).
		Msg("docker service deployment started")

	j.Deps.BroadcastToTeam(j.Payload.TeamID, "docker_service.deploying", map[string]any{
		"docker_service_id": j.service.ID,
		"deployment_id":     deployment.ID,
		"task_id":           taskModel.ID,
		"status":            "deploying",
	})

	return nil
}

func (j *DeployDockerServiceJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).
		Str("docker_service_id", j.Payload.DockerServiceID).
		Msg("failed to deploy docker service")

	j.Deps.DB.Model(&models.DockerService{}).
		Where("id = ?", j.Payload.DockerServiceID).
		Update("status", dockertypes.ServiceStatusFailed)

	j.Deps.BroadcastToTeam(j.Payload.TeamID, "docker_service.deploy_failed", map[string]any{
		"docker_service_id": j.Payload.DockerServiceID,
		"error":             err.Error(),
	})
}

func (j *DeployDockerServiceJob) loadServer(ctx context.Context) (*servermodels.Server, error) {
	var server servermodels.Server

	err := j.Deps.DB.WithContext(ctx).
		Preload("Services").
		First(&server, "id = ?", j.Payload.ServerID).Error
	if err != nil {
		return nil, err
	}

	return &server, nil
}

func (j *DeployDockerServiceJob) buildDeployConfig() tasks.DeployConfig {
	config := tasks.DeployConfig{
		ContainerName: j.service.ContainerName,
		Image:         j.service.Image,
		RestartPolicy: j.service.RestartPolicy.String(),
	}

	if j.service.Command != nil {
		config.Command = *j.service.Command
	}

	if j.service.CPULimit != nil {
		config.CPULimit = fmt.Sprintf("%.2f", *j.service.CPULimit)
	}

	if j.service.MemoryLimit != nil {
		config.MemoryLimit = fmt.Sprintf("%d", *j.service.MemoryLimit)
	}

	// Load registry credentials if present
	if j.service.Registry != nil {
		config.RegistryURL = j.service.Registry.URL
		config.RegistryUsername = j.service.Registry.Username
		config.RegistryPassword = j.service.Registry.Password.String()
	}

	// Map env vars
	for _, ev := range j.service.EnvVars {
		config.EnvVars = append(config.EnvVars, tasks.EnvVar{
			Key:   ev.Key,
			Value: ev.Value,
		})
	}

	// Map volumes
	for _, v := range j.service.Volumes {
		config.Volumes = append(config.Volumes, tasks.Volume{
			Source:   v.Source,
			Target:   v.Target,
			ReadOnly: v.ReadOnly,
		})
	}

	// Map ports
	for _, p := range j.service.Ports {
		config.Ports = append(config.Ports, tasks.Port{
			HostPort:      p.HostPort,
			ContainerPort: p.ContainerPort,
			Protocol:      p.Protocol.String(),
		})
	}

	return config
}

// NewDeployDockerServiceTask creates an asynq task for deploying a docker service.
func NewDeployDockerServiceTask(dockerServiceID, teamID, serverID string) error {
	return deps.Dispatch(TypeDeployDockerService, DeployDockerServicePayload{
		DockerServiceID: dockerServiceID,
		TeamID:          teamID,
		ServerID:        serverID,
	})
}
