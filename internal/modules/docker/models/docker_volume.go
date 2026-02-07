package models

import (
	"github.com/kkz6/launch-go/internal/modules/docker/types"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

type DockerVolume struct {
	basemodels.BaseModel

	DockerServiceID string          `gorm:"column:docker_service_id;type:char(26);not null;index" json:"docker_service_id"`
	MountType       types.MountType `gorm:"type:varchar(50);not null;default:volume" json:"mount_type"`
	Source          string          `gorm:"type:varchar(500);not null" json:"source"`
	Target          string          `gorm:"type:varchar(500);not null" json:"target"`
	ReadOnly        bool            `gorm:"type:boolean;not null;default:false" json:"read_only"`
}

func (DockerVolume) TableName() string { return "docker_service_volumes" }
