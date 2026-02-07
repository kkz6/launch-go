package models

import (
	"time"

	"github.com/kkz6/launch-go/internal/modules/docker/types"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

type DockerDeployment struct {
	basemodels.BaseModel

	DockerServiceID string                  `gorm:"column:docker_service_id;type:char(26);not null;index" json:"docker_service_id"`
	Image           string                  `gorm:"type:varchar(500);not null" json:"image"`
	Status          types.DeploymentStatus  `gorm:"type:varchar(50);not null;default:pending" json:"status"`
	Trigger         types.DeploymentTrigger `gorm:"type:varchar(50);not null;default:manual" json:"trigger"`
	Log             *string                 `gorm:"type:text" json:"log,omitempty"`
	StartedAt       *time.Time              `gorm:"type:timestamp null" json:"started_at,omitempty"`
	FinishedAt      *time.Time              `gorm:"type:timestamp null" json:"finished_at,omitempty"`

	// Relations
	DockerService *DockerService `gorm:"foreignKey:DockerServiceID" json:"docker_service,omitempty"`
}

func (DockerDeployment) TableName() string { return "docker_deployments" }
