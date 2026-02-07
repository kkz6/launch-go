package models

import (
	"github.com/kkz6/launch-go/internal/modules/docker/types"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

type DockerPort struct {
	basemodels.BaseModel

	DockerServiceID string         `gorm:"column:docker_service_id;type:char(26);not null;index" json:"docker_service_id"`
	HostPort        int            `gorm:"type:int;not null" json:"host_port"`
	ContainerPort   int            `gorm:"type:int;not null" json:"container_port"`
	Protocol        types.Protocol `gorm:"type:varchar(10);not null;default:tcp" json:"protocol"`
}

func (DockerPort) TableName() string { return "docker_service_ports" }
