package models

import (
	"time"

	"github.com/kkz6/launch-go/internal/modules/dockerapp/types"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

// App is a containerised application running on a Docker server.
// Single instance per (server, name); replicas are explicitly out of
// scope for this platform.
type App struct {
	basemodels.BaseModel
	basemodels.TeamScoped
	basemodels.ServerScoped

	// Name is the slug-safe identifier the user picks. Used to derive
	// the container name and named-volume prefixes.
	Name string `gorm:"type:varchar(64);not null;index" json:"name"`

	// Source identifies where the image comes from. "image" pulls a
	// pre-built image; "compose" runs an inline docker compose stack.
	Source types.Source `gorm:"type:varchar(16);not null" json:"source"`

	// Image and Tag are the docker image reference for source=image.
	// Combined as "<image>:<tag>" at deploy time. NULL when source=compose.
	Image string `gorm:"type:varchar(255)" json:"image"`
	Tag   string `gorm:"type:varchar(255);not null;default:latest" json:"tag"`

	// ComposeYAML is the inline docker-compose file for source=compose.
	ComposeYAML *string `gorm:"column:compose_yaml;type:longtext" json:"compose_yaml,omitempty"`
	// ComposeEnv is the inline .env content for source=compose. Encrypted at
	// rest because it commonly carries credentials.
	ComposeEnv *dbtype.EncryptedString `gorm:"column:compose_env;type:longtext" json:"-"`

	// RegistryCredentialID points at a docker_registry_credentials row
	// when the image is private. NULL means anonymous pull.
	RegistryCredentialID *string `gorm:"column:registry_credential_id;type:char(26);index" json:"registry_credential_id,omitempty"`

	// RestartPolicy is passed to `docker run --restart`.
	RestartPolicy types.RestartPolicy `gorm:"type:varchar(32);not null;default:unless-stopped" json:"restart_policy"`

	// Status is the lifecycle state. UI uses it to drive enabled buttons.
	Status types.Status `gorm:"type:varchar(32);not null;index" json:"status"`

	// LastError records the most recent failure. Surfaced in the UI.
	LastError *string `gorm:"column:last_error;type:text" json:"last_error,omitempty"`

	// TaskID is the most recent task for this app, used to wire the log
	// viewer to past task output.
	TaskID *string `gorm:"column:task_id;type:char(26);index" json:"task_id,omitempty"`

	// DeployedAt is the timestamp of the last successful deploy.
	DeployedAt *time.Time `gorm:"column:deployed_at;type:timestamp null" json:"deployed_at,omitempty"`

	// Relations — populated when the caller explicitly preloads them.
	EnvVars []EnvVar `gorm:"foreignKey:AppID" json:"env_vars,omitempty"`
	Ports   []Port   `gorm:"foreignKey:AppID" json:"ports,omitempty"`
	Volumes []Volume `gorm:"foreignKey:AppID" json:"volumes,omitempty"`
	Domains []Domain `gorm:"foreignKey:AppID" json:"domains,omitempty"`
}

// TableName overrides the default GORM table name.
func (App) TableName() string { return "docker_apps" }

// Container returns the deterministic docker container name on the host.
func (a *App) Container() string { return types.DefaultContainerName(a.Name) }

// ImageRef returns the "image:tag" string used by docker pull/run for
// source=image apps. Empty for compose apps.
func (a *App) ImageRef() string {
	if a.Source != types.SourceImage {
		return ""
	}
	tag := a.Tag
	if tag == "" {
		tag = "latest"
	}
	return a.Image + ":" + tag
}

// ComposeProject returns the deterministic docker-compose project name.
// Used as the -p flag when running docker compose so multiple apps don't
// collide on a host.
func (a *App) ComposeProject() string { return "launch-app-" + a.Name }

// ComposeYAMLValue returns the compose document, or empty string when not set.
func (a *App) ComposeYAMLValue() string {
	if a.ComposeYAML == nil {
		return ""
	}
	return *a.ComposeYAML
}

// ComposeEnvValue returns the decoded .env content, or empty string when not set.
func (a *App) ComposeEnvValue() string {
	if a.ComposeEnv == nil {
		return ""
	}
	return string(*a.ComposeEnv)
}
