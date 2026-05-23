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
	EnvFile        *string                       `gorm:"column:env_file;type:longtext" json:"env_file,omitempty"`
	Status         dockertypes.ApplicationStatus `gorm:"type:varchar(32);not null;default:idle" json:"status"`
	LastDeployedAt *time.Time                    `gorm:"column:last_deployed_at;type:timestamp null" json:"last_deployed_at,omitempty"`
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
