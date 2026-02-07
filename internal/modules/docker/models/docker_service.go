package models

import (
	"time"

	"github.com/kkz6/launch-go/internal/modules/docker/types"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

type DockerService struct {
	basemodels.BaseModel
	basemodels.TeamScoped
	basemodels.ServerScoped
	basemodels.UserScoped

	Name          string              `gorm:"type:varchar(255);not null" json:"name"`
	ContainerName string              `gorm:"type:varchar(255);not null" json:"container_name"`
	Kind          types.ServiceKind   `gorm:"type:varchar(50);not null;default:application" json:"kind"`
	Status        types.ServiceStatus `gorm:"type:varchar(50);not null;default:pending" json:"status"`
	Image         string              `gorm:"type:varchar(500);not null" json:"image"`
	RegistryID    *string             `gorm:"type:char(26)" json:"registry_id,omitempty"`
	Command       *string             `gorm:"type:text" json:"command,omitempty"`
	Entrypoint    *string             `gorm:"type:text" json:"entrypoint,omitempty"`
	RestartPolicy types.RestartPolicy `gorm:"type:varchar(50);not null;default:unless-stopped" json:"restart_policy"`
	CPULimit      *float64            `gorm:"type:decimal(5,2)" json:"cpu_limit,omitempty"`
	MemoryLimit   *int                `gorm:"type:int" json:"memory_limit,omitempty"`

	HealthCheckCmd      *string `gorm:"type:text" json:"health_check_cmd,omitempty"`
	HealthCheckInterval int     `gorm:"type:int;default:30" json:"health_check_interval"`
	HealthCheckTimeout  int     `gorm:"type:int;default:10" json:"health_check_timeout"`
	HealthCheckRetries  int     `gorm:"type:int;default:3" json:"health_check_retries"`

	DeployToken      string  `gorm:"type:varchar(64);not null" json:"-"`
	ComposeProjectID *string `gorm:"type:char(26)" json:"compose_project_id,omitempty"`

	InstalledAt          *time.Time `gorm:"type:timestamp null" json:"installed_at,omitempty"`
	InstallationFailedAt *time.Time `gorm:"type:timestamp null" json:"installation_failed_at,omitempty"`

	// Relations
	EnvVars        []DockerEnvVar        `gorm:"foreignKey:DockerServiceID" json:"env_vars,omitempty"`
	Volumes        []DockerVolume        `gorm:"foreignKey:DockerServiceID" json:"volumes,omitempty"`
	Ports          []DockerPort          `gorm:"foreignKey:DockerServiceID" json:"ports,omitempty"`
	Domains        []DockerDomain        `gorm:"foreignKey:DockerServiceID" json:"domains,omitempty"`
	Deployments    []DockerDeployment    `gorm:"foreignKey:DockerServiceID" json:"deployments,omitempty"`
	Registry       *DockerRegistry       `gorm:"foreignKey:RegistryID" json:"registry,omitempty"`
	ComposeProject *DockerComposeProject `gorm:"foreignKey:ComposeProjectID" json:"compose_project,omitempty"`
}

func (DockerService) TableName() string { return "docker_services" }
