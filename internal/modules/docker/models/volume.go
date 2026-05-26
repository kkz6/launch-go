package models

import (
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

// ApplicationVolume is a mount attached to a docker workload at
// deploy time. Polymorphic by owner — every live row sets EXACTLY
// one of `ApplicationID` or `ComposeID` (enforced at the service
// layer). Three flavours (matches dokploy's mount kinds):
//
//   - bind:   `HostPath` is the directory on the docker server bind-
//             mounted into the container at `MountPath`. Application
//             deploy script renders `-v <host_path>:<mount_path>`;
//             compose deploys rely on the operator wiring it into
//             the YAML themselves.
//   - volume: `Name` is the docker-named volume that survives
//             container restarts. Application script renders
//             `-v <name>:<mount_path>`. The wire value "named" is
//             still accepted for backward compatibility with rows
//             created before this slice.
//   - file:   `Content` is written to the host at the deploy dir
//             (named by `FilePath`), then bind-mounted at
//             `MountPath`. Applications render
//             `-v <deploy_dir>/<file_path>:<mount_path>`; compose
//             writes it under `${STACK_DIR}/files/<file_path>` so
//             the YAML can reference it via
//             `./files/<file_path>:<mount_path>`.
//
// The type name stays `ApplicationVolume` rather than the more
// accurate `DockerVolume` to keep the diff small — every reference
// would have to move otherwise. The table name `docker_application_
// volumes` is similarly preserved for the same reason.
//
// Uniqueness is enforced per-owner at the service layer:
// (application_id, name) and (compose_id, name) both unique among
// live rows.
type ApplicationVolume struct {
	basemodels.BaseModel
	basemodels.SoftDeleteModel
	// One of ApplicationID / ComposeID is set per row. Both are
	// nullable so the same table backs application-owned and
	// compose-owned mounts without a second table or a polymorphic-
	// type column. Indexes on both columns so per-owner List queries
	// stay cheap.
	ApplicationID *string `gorm:"column:application_id;type:char(26);index" json:"application_id,omitempty"`
	ComposeID     *string `gorm:"column:compose_id;type:char(26);index" json:"compose_id,omitempty"`
	Name          string  `gorm:"type:varchar(255);not null" json:"name"`
	MountPath     string  `gorm:"column:mount_path;type:varchar(512);not null" json:"mount_path"`
	Type          string  `gorm:"type:varchar(32);not null" json:"type"` // "bind" | "volume" | "file" (legacy: "named")
	HostPath      *string `gorm:"column:host_path;type:varchar(512)" json:"host_path,omitempty"`
	// Content is the body written for type=file. Read-on-demand only;
	// `json:"-"` keeps it off the list response so file mounts with
	// large config bodies don't bloat the payload. The detail endpoint
	// exposes it via the response DTO when needed.
	Content *string `gorm:"column:content;type:longtext" json:"-"`
	// FilePath is the on-host filename written for type=file. For
	// applications: relative to the application's deploy directory.
	// For compose: relative to `${STACK_DIR}/files/`.
	FilePath *string `gorm:"column:file_path;type:varchar(512)" json:"file_path,omitempty"`
}

func (ApplicationVolume) TableName() string { return "docker_application_volumes" }
