package models

import (
	"time"

	dockertypes "github.com/kkz6/launch-go/internal/modules/docker/types"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

// Application is a single-container docker workload.
//
// Phase 1: model only. The repository, service, and worker pipeline land
// in later phases — but the table needs to exist so foreign keys from
// related tables (env vars, domains, …) are valid.
type Application struct {
	basemodels.BaseModel
	basemodels.TeamScoped
	basemodels.ServerScoped
	basemodels.SoftDeleteModel
	ProjectID      string                      `gorm:"column:project_id;type:char(26);not null;index" json:"project_id"`
	Name           string                      `gorm:"type:varchar(255);not null" json:"name"`
	SourceType     dockertypes.SourceType      `gorm:"column:source_type;type:varchar(32);not null" json:"source_type"`
	SourceConfig   dbtype.JSONMap              `gorm:"column:source_config;type:json" json:"source_config,omitempty"`
	BuildType      *dockertypes.BuildType      `gorm:"column:build_type;type:varchar(32)" json:"build_type,omitempty"`
	BuildConfig    dbtype.JSONMap              `gorm:"column:build_config;type:json" json:"build_config,omitempty"`
	Status         dockertypes.ApplicationStatus `gorm:"type:varchar(32);not null;default:idle" json:"status"`
	ContainerID    *string                       `gorm:"column:container_id;type:varchar(255)" json:"container_id,omitempty"`
	LastDeployedAt *time.Time                    `gorm:"column:last_deployed_at;type:timestamp null" json:"last_deployed_at,omitempty"`

	// InternalPort is the port the container's service listens on inside
	// the container. Traefik routes traffic to this port over the
	// launch-network. Default 80 in the migration so existing rows pick
	// up a sane value.
	InternalPort int `gorm:"column:internal_port;type:int;not null;default:80" json:"internal_port"`

	// --- Docker image authentication ---------------------------------
	//
	// Pick ONE of two paths (service layer enforces at-most-one-of):
	//
	// 1. Saved-credential path: `RegistryCredentialID` references a
	//    `registry_credentials` row managed in Settings → Connections.
	//    The deploy script reads the row at deploy time, decrypts the
	//    secrets, and runs `docker login` before pulling.
	//
	// 2. Inline path: `RegistryUsername` + `RegistryPassword` carry
	//    credentials owned by this application alone. Useful for
	//    one-off private images where saving a reusable credential
	//    isn't worth it. Password is encrypted at rest via the same
	//    `dbtype.EncryptedString` path env-var values use.
	//
	// Both empty / nil → public image, no `docker login` step. The
	// FK on RegistryCredentialID is ON DELETE SET NULL (see 0037) so
	// deleting a saved credential disconnects but doesn't tombstone
	// the application — the operator gets a chance to re-attach or
	// switch to inline.
	RegistryCredentialID *string                `gorm:"column:registry_credential_id;type:char(26);index" json:"registry_credential_id,omitempty"`
	RegistryUsername     *string                `gorm:"column:registry_username;type:varchar(255)" json:"registry_username,omitempty"`
	RegistryPassword     dbtype.EncryptedString `gorm:"column:registry_password;type:longtext" json:"-"`
}

func (Application) TableName() string { return "docker_applications" }

// Compose is a multi-container workload defined by a docker-compose file.
type Compose struct {
	basemodels.BaseModel
	basemodels.TeamScoped
	basemodels.ServerScoped
	basemodels.SoftDeleteModel
	ProjectID         string                        `gorm:"column:project_id;type:char(26);not null;index" json:"project_id"`
	Name              string                        `gorm:"type:varchar(255);not null" json:"name"`
	ComposeSourceType string                        `gorm:"column:compose_source_type;type:varchar(32);not null" json:"compose_source_type"`
	SourceConfig      dbtype.JSONMap                `gorm:"column:source_config;type:json" json:"source_config,omitempty"`
	ComposeFilePath   *string                       `gorm:"column:compose_file_path;type:varchar(512)" json:"compose_file_path,omitempty"`
	RawYAML           *string                       `gorm:"column:raw_yaml;type:longtext" json:"raw_yaml,omitempty"`
	// EnvFile is the body of the `.env` file written next to the
	// compose file on each deploy. Compose reads it automatically for
	// ${VAR} substitution and propagates matching keys into service
	// environment blocks without explicit values. NULL = no env file
	// (default). UI surfaces this as the compose Environment subtab.
	EnvFile *string `gorm:"column:env_file;type:longtext" json:"env_file,omitempty"`
	// RunCommand overrides the docker command suffix the deploy script
	// runs. When NULL, the default command is rendered (
	// `compose -p <name> -f <file> up -d --build --remove-orphans`).
	// When set, the deploy script runs `docker <run_command>` verbatim
	// — the operator owns the whole tail including the `compose ...`
	// shape. Matches dokploy's Run Command feature. UI surfaces this
	// on the Advanced subtab.
	RunCommand     *string                       `gorm:"column:run_command;type:longtext" json:"run_command,omitempty"`
	Status         dockertypes.ApplicationStatus `gorm:"type:varchar(32);not null;default:idle" json:"status"`
	LastDeployedAt *time.Time                    `gorm:"column:last_deployed_at;type:timestamp null" json:"last_deployed_at,omitempty"`

	// RegistryCredentials are the 0..N saved registry logins this
	// stack should authenticate against before pulling images. The
	// deploy job runs `docker login` for each one before
	// `docker compose up`. Compose stacks routinely reference images
	// from multiple registries (ghcr + a private quay), which is why
	// this is many-to-many rather than the singular pointer the
	// application path uses.
	//
	// Many2many backed by docker_compose_registry_credentials. `-`
	// in the JSON tag suppresses the relationship on the row's own
	// serialization — the service layer maps the join into a curated
	// response DTO (id + name + registry_url, never the secrets).
	RegistryCredentials []RegistryCredential `gorm:"many2many:docker_compose_registry_credentials;joinForeignKey:ComposeID;joinReferences:RegistryCredentialID" json:"-"`
}

func (Compose) TableName() string { return "docker_composes" }

// Database is a managed-database container running on a docker server.
type Database struct {
	basemodels.BaseModel
	basemodels.TeamScoped
	basemodels.ServerScoped
	basemodels.SoftDeleteModel
	ProjectID     string                        `gorm:"column:project_id;type:char(26);not null;index" json:"project_id"`
	Name          string                        `gorm:"type:varchar(255);not null" json:"name"`
	Engine        dockertypes.DatabaseEngine    `gorm:"type:varchar(32);not null" json:"engine"`
	EngineVersion string                        `gorm:"column:engine_version;type:varchar(32);not null" json:"engine_version"`
	ImageTag      *string                       `gorm:"column:image_tag;type:varchar(255)" json:"image_tag,omitempty"`
	ExternalPort  *int                          `gorm:"column:external_port;type:int" json:"external_port,omitempty"`
	Credentials   dbtype.EncryptedString        `gorm:"type:longtext" json:"-"`
	Status        dockertypes.ApplicationStatus `gorm:"type:varchar(32);not null;default:starting" json:"status"`
	// BuildConfig holds Advanced-subtab knobs (restart_policy,
	// cpu_limit, memory_limit, cpu_reservation, memory_reservation).
	// Mirrors docker_applications.build_config so the same DTO shape
	// works on both sides.
	BuildConfig dbtype.JSONMap `gorm:"column:build_config;type:longtext" json:"build_config,omitempty"`
}

func (Database) TableName() string { return "docker_databases" }
