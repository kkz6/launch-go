package models

import (
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

// ApplicationVolume is a mount attached to the container at deploy
// time. Three flavours (matches dokploy's mount kinds):
//
//   - bind:   `HostPath` is the directory on the docker server bind-
//             mounted into the container at `MountPath`. Script renders
//             `-v <host_path>:<mount_path>`.
//   - volume: `Name` is the docker-named volume that survives container
//             restarts. Script renders `-v <name>:<mount_path>`. The
//             value "named" is still accepted on the wire for backward
//             compatibility with rows created before this slice.
//   - file:   `Content` is written to a file on the host at the deploy
//             dir (named by `FilePath`), then bind-mounted at
//             `MountPath`. Useful for config files (nginx.conf, .env)
//             the user wants versioned in the platform rather than baked
//             into the image. Script renders `-v <deploy_dir>/<file_path>:<mount_path>`.
//
// (application_id, name) is unique among live rows.
type ApplicationVolume struct {
	basemodels.BaseModel
	basemodels.SoftDeleteModel
	ApplicationID string  `gorm:"column:application_id;type:char(26);not null;index" json:"application_id"`
	Name          string  `gorm:"type:varchar(255);not null" json:"name"`
	MountPath     string  `gorm:"column:mount_path;type:varchar(512);not null" json:"mount_path"`
	Type          string  `gorm:"type:varchar(32);not null" json:"type"` // "bind" | "volume" | "file" (legacy: "named")
	HostPath      *string `gorm:"column:host_path;type:varchar(512)" json:"host_path,omitempty"`
	// Content is the body written for type=file. Read-on-demand only;
	// `json:"-"` keeps it off the list response so file mounts with
	// large config bodies don't bloat the payload. The detail endpoint
	// exposes it via the response DTO when needed.
	Content *string `gorm:"column:content;type:longtext" json:"-"`
	// FilePath is the on-host filename written for type=file (relative
	// to the application's deploy directory). e.g. "nginx.conf".
	FilePath *string `gorm:"column:file_path;type:varchar(512)" json:"file_path,omitempty"`
}

func (ApplicationVolume) TableName() string { return "docker_application_volumes" }
