package models

import (
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

// ApplicationVolume is a mount attached to the container at deploy
// time. Two flavours:
//
//   - named:  `Name` is the docker-named volume that survives container
//             restarts. The deploy script passes it as `-v <name>:<mount_path>`.
//   - bind:   `HostPath` is the directory on the docker server bind-
//             mounted into the container at `MountPath`. The deploy
//             script passes it as `-v <host_path>:<mount_path>`.
//
// (application_id, name) is unique among live rows.
type ApplicationVolume struct {
	basemodels.BaseModel
	basemodels.SoftDeleteModel
	ApplicationID string  `gorm:"column:application_id;type:char(26);not null;index" json:"application_id"`
	Name          string  `gorm:"type:varchar(255);not null" json:"name"`
	MountPath     string  `gorm:"column:mount_path;type:varchar(512);not null" json:"mount_path"`
	Type          string  `gorm:"type:varchar(32);not null" json:"type"` // "named" | "bind"
	HostPath      *string `gorm:"column:host_path;type:varchar(512)" json:"host_path,omitempty"`
}

func (ApplicationVolume) TableName() string { return "docker_application_volumes" }
