package models

import basemodels "github.com/kkz6/launch-go/internal/pkg/models"

// Volume mounts a named docker volume at a path inside the container.
// We use named volumes only (not bind mounts) so the host path stays
// hidden from the user and the volume survives container recreation.
type Volume struct {
	basemodels.BaseModel

	AppID string `gorm:"column:app_id;type:char(26);not null;index" json:"app_id"`
	// Name is a short identifier the user picks; the actual volume name
	// on the host is derived as launch-app-<app>-<name>.
	Name string `gorm:"type:varchar(64);not null" json:"name"`
	// MountPath is where it mounts inside the container.
	MountPath string `gorm:"column:mount_path;type:varchar(255);not null" json:"mount_path"`
}

// TableName overrides the default GORM table name.
func (Volume) TableName() string { return "docker_app_volumes" }
