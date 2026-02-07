package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/kkz6/launch-go/internal/modules/docker/jobs"
	"github.com/kkz6/launch-go/internal/modules/docker/models"
	"github.com/kkz6/launch-go/internal/modules/docker/repositories"
	"github.com/kkz6/launch-go/internal/modules/docker/types"
	"github.com/kkz6/launch-go/internal/pkg/service"
	"github.com/kkz6/launch-go/internal/pkg/util"
)

// ServiceDeps holds dependencies for the docker service
type ServiceDeps struct {
	service.Dependencies
	Repos *repositories.Registry
}

// DockerService handles docker service business logic
type DockerService struct {
	service.Base
	repos *repositories.Registry
}

// NewDockerService creates a new DockerService
func NewDockerService(deps ServiceDeps) *DockerService {
	return &DockerService{
		Base:  service.NewBase(deps.Dependencies),
		repos: deps.Repos,
	}
}

// CreateService creates a new docker service
func (s *DockerService) CreateService(ctx context.Context, serverID, teamID, userID, name, image string, kind types.ServiceKind) (*models.DockerService, error) {
	containerName := generateContainerName(name)

	deployToken, err := generateDeployToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate deploy token: %w", err)
	}

	svc := &models.DockerService{
		Name:          name,
		ContainerName: containerName,
		Kind:          kind,
		Status:        types.ServiceStatusPending,
		Image:         image,
		RestartPolicy: types.RestartPolicyUnlessStopped,
		DeployToken:   deployToken,
	}
	svc.ServerID = serverID
	svc.TeamID = teamID
	svc.UserID = userID

	if err := s.repos.DockerService().Create(ctx, svc); err != nil {
		return nil, fmt.Errorf("failed to create docker service: %w", err)
	}

	return svc, nil
}

// GetService retrieves a docker service by ID
func (s *DockerService) GetService(ctx context.Context, id string) (*models.DockerService, error) {
	return s.repos.DockerService().FindByID(ctx, id)
}

// ListServices retrieves all docker services for a server
func (s *DockerService) ListServices(ctx context.Context, serverID string) ([]models.DockerService, error) {
	return s.repos.DockerService().FindByServerID(ctx, serverID)
}

// UpdateService saves changes to a docker service
func (s *DockerService) UpdateService(ctx context.Context, svc *models.DockerService) error {
	return s.repos.DockerService().Update(ctx, svc)
}

// DeployService creates a deployment record and dispatches the deploy job
func (s *DockerService) DeployService(ctx context.Context, id, teamID string) (*models.DockerDeployment, error) {
	svc, err := s.repos.DockerService().FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to find docker service: %w", err)
	}

	deployment := &models.DockerDeployment{
		DockerServiceID: svc.ID,
		Image:           svc.Image,
		Status:          types.DeploymentStatusPending,
		Trigger:         types.DeploymentTriggerManual,
	}

	if err := s.repos.DockerDeployment().Create(ctx, deployment); err != nil {
		return nil, fmt.Errorf("failed to create deployment: %w", err)
	}

	if err := jobs.NewDeployDockerServiceTask(svc.ID, teamID, svc.ServerID); err != nil {
		s.LogError(err, "Failed to dispatch deploy job", "docker_service_id", svc.ID)
		return nil, fmt.Errorf("failed to dispatch deploy job: %w", err)
	}

	return deployment, nil
}

// StopService dispatches a stop job for the docker service
func (s *DockerService) StopService(ctx context.Context, svc *models.DockerService) error {
	return jobs.NewManageDockerServiceTask(svc.ID, svc.TeamID, svc.ServerID, "stop")
}

// StartService dispatches a start job for the docker service
func (s *DockerService) StartService(ctx context.Context, svc *models.DockerService) error {
	return jobs.NewManageDockerServiceTask(svc.ID, svc.TeamID, svc.ServerID, "start")
}

// RestartService dispatches a restart job for the docker service
func (s *DockerService) RestartService(ctx context.Context, svc *models.DockerService) error {
	return jobs.NewManageDockerServiceTask(svc.ID, svc.TeamID, svc.ServerID, "restart")
}

// DeployServiceViaWebhook creates a deployment triggered by webhook
func (s *DockerService) DeployServiceViaWebhook(ctx context.Context, svc *models.DockerService) (*models.DockerDeployment, error) {
	deployment := &models.DockerDeployment{
		DockerServiceID: svc.ID,
		Image:           svc.Image,
		Status:          types.DeploymentStatusPending,
		Trigger:         types.DeploymentTriggerWebhook,
	}

	if err := s.repos.DockerDeployment().Create(ctx, deployment); err != nil {
		return nil, fmt.Errorf("failed to create deployment: %w", err)
	}

	if err := jobs.NewDeployDockerServiceTask(svc.ID, svc.TeamID, svc.ServerID); err != nil {
		s.LogError(err, "Failed to dispatch webhook deploy job", "docker_service_id", svc.ID)
		return nil, fmt.Errorf("failed to dispatch deploy job: %w", err)
	}

	return deployment, nil
}

// ImportCompose parses a docker-compose.yml and creates a compose project with services
func (s *DockerService) ImportCompose(ctx context.Context, serverID, teamID, userID, projectName, content string) (*models.DockerComposeProject, error) {
	parsed, err := ParseComposeFile(content)
	if err != nil {
		return nil, err
	}

	// Create compose project
	project := &models.DockerComposeProject{
		Name:       projectName,
		RawCompose: content,
	}
	project.TeamID = teamID
	project.ServerID = serverID

	if err := s.repos.DB().WithContext(ctx).Create(project).Error; err != nil {
		return nil, fmt.Errorf("failed to create compose project: %w", err)
	}

	// Create services from parsed compose file
	for _, p := range parsed {
		containerName := generateContainerName(p.Name)

		deployToken, err := generateDeployToken()
		if err != nil {
			return nil, fmt.Errorf("failed to generate deploy token: %w", err)
		}

		svc := &models.DockerService{
			Name:             p.Name,
			ContainerName:    containerName,
			Kind:             p.Kind,
			Status:           types.ServiceStatusPending,
			Image:            p.Image,
			Command:          p.Command,
			Entrypoint:       p.Entrypoint,
			RestartPolicy:    p.RestartPolicy,
			CPULimit:         p.CPULimit,
			MemoryLimit:      p.MemoryLimit,
			DeployToken:      deployToken,
			ComposeProjectID: &project.ID,
		}
		svc.ServerID = serverID
		svc.TeamID = teamID
		svc.UserID = userID

		if err := s.repos.DockerService().Create(ctx, svc); err != nil {
			return nil, fmt.Errorf("failed to create service %s: %w", p.Name, err)
		}

		// Create env vars
		for _, ev := range p.EnvVars {
			envVar := &models.DockerEnvVar{
				DockerServiceID: svc.ID,
				Key:             ev.Key,
				Value:           ev.Value,
				IsSecret:        ev.IsSecret,
			}

			if err := s.repos.DB().WithContext(ctx).Create(envVar).Error; err != nil {
				return nil, fmt.Errorf("failed to create env var: %w", err)
			}
		}

		// Create ports
		for _, port := range p.Ports {
			portModel := &models.DockerPort{
				DockerServiceID: svc.ID,
				HostPort:        port.HostPort,
				ContainerPort:   port.ContainerPort,
				Protocol:        port.Protocol,
			}

			if err := s.repos.DB().WithContext(ctx).Create(portModel).Error; err != nil {
				return nil, fmt.Errorf("failed to create port: %w", err)
			}
		}

		// Create volumes
		for _, vol := range p.Volumes {
			volModel := &models.DockerVolume{
				DockerServiceID: svc.ID,
				MountType:       vol.MountType,
				Source:          vol.Source,
				Target:          vol.Target,
				ReadOnly:        vol.ReadOnly,
			}

			if err := s.repos.DB().WithContext(ctx).Create(volModel).Error; err != nil {
				return nil, fmt.Errorf("failed to create volume: %w", err)
			}
		}
	}

	// Reload with relations
	var result models.DockerComposeProject
	if err := s.repos.DB().WithContext(ctx).
		Preload("Services").
		First(&result, "id = ?", project.ID).Error; err != nil {
		return nil, err
	}

	return &result, nil
}

// DeleteService removes a docker service by ID
func (s *DockerService) DeleteService(ctx context.Context, id string) error {
	return s.repos.DockerService().Delete(ctx, id)
}

func generateContainerName(name string) string {
	shortID := util.NewULID()[:8]
	sanitized := strings.ToLower(strings.ReplaceAll(name, " ", "-"))

	return fmt.Sprintf("%s-%s", sanitized, shortID)
}

func generateDeployToken() (string, error) {
	b := make([]byte, 32)

	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	return hex.EncodeToString(b), nil
}
