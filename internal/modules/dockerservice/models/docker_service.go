package models

import (
	"github.com/kkz6/launch-go/internal/modules/dockerservice/types"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

// DockerService represents a managed database/cache service running in a
// Docker container on a server. Single instance per (server, kind).
type DockerService struct {
	basemodels.BaseModel
	basemodels.TeamScoped
	basemodels.ServerScoped

	Kind   types.Kind   `gorm:"type:varchar(32);not null;index" json:"kind"`
	Status types.Status `gorm:"type:varchar(32);not null;index" json:"status"`

	// Image is the docker image:tag to run. Defaults from Kind.DefaultImage()
	// when the caller does not supply one at install time.
	Image string `gorm:"type:varchar(255);not null" json:"image"`

	// Container is the deterministic docker container name on the host.
	Container string `gorm:"type:varchar(255);not null" json:"container"`

	// Volume is the named docker volume that holds persistent data.
	Volume string `gorm:"type:varchar(255);not null" json:"volume"`

	// DatabaseName is the initial database that the engine creates on
	// first start. Empty for Redis.
	DatabaseName *string `gorm:"column:database_name;type:varchar(255)" json:"database_name,omitempty"`

	// Username is the superuser/AUTH user. For Postgres/MySQL this is the
	// admin user; for Redis it is unused (AUTH is password-only).
	Username *string `gorm:"type:varchar(255)" json:"username,omitempty"`

	// Password is stored in plain text at rest the same way other engine
	// credentials are stored on this platform; rotate by uninstall +
	// reinstall.
	Password string `gorm:"type:varchar(255);not null" json:"-"`

	// LastError records the most recent failure (install/uninstall/lifecycle).
	LastError *string `gorm:"column:last_error;type:text" json:"last_error,omitempty"`

	// TaskID points at the most recent in-flight or finished task for
	// this service, used to surface logs in the UI.
	TaskID *string `gorm:"column:task_id;type:char(26);index" json:"task_id,omitempty"`
}

// TableName overrides the default GORM table name.
func (DockerService) TableName() string {
	return "docker_services"
}

// IsRunning reports whether the underlying container is expected to be up.
func (m *DockerService) IsRunning() bool {
	return m.Status == types.StatusRunning
}

// IsStopped reports whether the service is intentionally stopped.
func (m *DockerService) IsStopped() bool {
	return m.Status == types.StatusStopped
}

// IsBusy reports whether a lifecycle job is currently advancing this
// service (i.e. install/uninstall in progress).
func (m *DockerService) IsBusy() bool {
	return m.Status == types.StatusInstalling || m.Status == types.StatusPending
}
