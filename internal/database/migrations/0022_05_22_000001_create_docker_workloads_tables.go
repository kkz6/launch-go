package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0022_05_22_000001_create_docker_workloads_tables",
		Name:      "Create docker workload tables",
		Timestamp: time.Date(2022, 5, 22, 0, 0, 1, 0, time.UTC),
		Up:        createDockerWorkloadsTablesUp,
		Down:      createDockerWorkloadsTablesDown,
	})
}

// docker_applications --------------------------------------------------------

type dockerApplicationMigration struct {
	ID             string         `gorm:"type:char(26);primaryKey"`
	TeamID         string         `gorm:"column:team_id;type:char(26);not null;index"`
	ServerID       string         `gorm:"column:server_id;type:char(26);not null;index"`
	ProjectID      string         `gorm:"column:project_id;type:char(26);not null;index"`
	Name           string         `gorm:"type:varchar(255);not null"`
	SourceType     string         `gorm:"column:source_type;type:varchar(32);not null"`
	SourceConfig   *string        `gorm:"column:source_config;type:json"`
	BuildType      *string        `gorm:"column:build_type;type:varchar(32)"`
	BuildConfig    *string        `gorm:"column:build_config;type:json"`
	Status         string         `gorm:"type:varchar(32);not null;default:idle"`
	ContainerID    *string        `gorm:"column:container_id;type:varchar(255)"`
	LastDeployedAt *time.Time     `gorm:"column:last_deployed_at;type:timestamp null"`
	CreatedAt      *time.Time     `gorm:"type:timestamp null"`
	UpdatedAt      *time.Time     `gorm:"type:timestamp null"`
	DeletedAt      gorm.DeletedAt `gorm:"index"`
}

func (dockerApplicationMigration) TableName() string { return "docker_applications" }

type dockerApplicationWithProjectFK struct {
	ProjectID string                  `gorm:"column:project_id"`
	Project   *dockerProjectMigration `gorm:"foreignKey:ProjectID;references:ID;constraint:OnDelete:CASCADE"`
}

func (dockerApplicationWithProjectFK) TableName() string { return "docker_applications" }

type dockerApplicationWithServerFK struct {
	ServerID string           `gorm:"column:server_id"`
	Server   *serverMigration `gorm:"foreignKey:ServerID;references:ID;constraint:OnDelete:CASCADE"`
}

func (dockerApplicationWithServerFK) TableName() string { return "docker_applications" }

// docker_composes ------------------------------------------------------------

type dockerComposeMigration struct {
	ID                string         `gorm:"type:char(26);primaryKey"`
	TeamID            string         `gorm:"column:team_id;type:char(26);not null;index"`
	ServerID          string         `gorm:"column:server_id;type:char(26);not null;index"`
	ProjectID         string         `gorm:"column:project_id;type:char(26);not null;index"`
	Name              string         `gorm:"type:varchar(255);not null"`
	ComposeSourceType string         `gorm:"column:compose_source_type;type:varchar(32);not null"`
	SourceConfig      *string        `gorm:"column:source_config;type:json"`
	ComposeFilePath   *string        `gorm:"column:compose_file_path;type:varchar(512)"`
	RawYAML           *string        `gorm:"column:raw_yaml;type:longtext"`
	Status            string         `gorm:"type:varchar(32);not null;default:idle"`
	LastDeployedAt    *time.Time     `gorm:"column:last_deployed_at;type:timestamp null"`
	CreatedAt         *time.Time     `gorm:"type:timestamp null"`
	UpdatedAt         *time.Time     `gorm:"type:timestamp null"`
	DeletedAt         gorm.DeletedAt `gorm:"index"`
}

func (dockerComposeMigration) TableName() string { return "docker_composes" }

type dockerComposeWithProjectFK struct {
	ProjectID string                  `gorm:"column:project_id"`
	Project   *dockerProjectMigration `gorm:"foreignKey:ProjectID;references:ID;constraint:OnDelete:CASCADE"`
}

func (dockerComposeWithProjectFK) TableName() string { return "docker_composes" }

type dockerComposeWithServerFK struct {
	ServerID string           `gorm:"column:server_id"`
	Server   *serverMigration `gorm:"foreignKey:ServerID;references:ID;constraint:OnDelete:CASCADE"`
}

func (dockerComposeWithServerFK) TableName() string { return "docker_composes" }

// docker_databases -----------------------------------------------------------

type dockerDatabaseMigration struct {
	ID            string         `gorm:"type:char(26);primaryKey"`
	TeamID        string         `gorm:"column:team_id;type:char(26);not null;index"`
	ServerID      string         `gorm:"column:server_id;type:char(26);not null;index"`
	ProjectID     string         `gorm:"column:project_id;type:char(26);not null;index"`
	Name          string         `gorm:"type:varchar(255);not null"`
	Engine        string         `gorm:"type:varchar(32);not null"`
	EngineVersion string         `gorm:"column:engine_version;type:varchar(32);not null"`
	ImageTag      *string        `gorm:"column:image_tag;type:varchar(255)"`
	ExternalPort  *int           `gorm:"column:external_port;type:int"`
	Credentials   *string        `gorm:"type:longtext"`
	Status        string         `gorm:"type:varchar(32);not null;default:starting"`
	CreatedAt     *time.Time     `gorm:"type:timestamp null"`
	UpdatedAt     *time.Time     `gorm:"type:timestamp null"`
	DeletedAt     gorm.DeletedAt `gorm:"index"`
}

func (dockerDatabaseMigration) TableName() string { return "docker_databases" }

type dockerDatabaseWithProjectFK struct {
	ProjectID string                  `gorm:"column:project_id"`
	Project   *dockerProjectMigration `gorm:"foreignKey:ProjectID;references:ID;constraint:OnDelete:CASCADE"`
}

func (dockerDatabaseWithProjectFK) TableName() string { return "docker_databases" }

type dockerDatabaseWithServerFK struct {
	ServerID string           `gorm:"column:server_id"`
	Server   *serverMigration `gorm:"foreignKey:ServerID;references:ID;constraint:OnDelete:CASCADE"`
}

func (dockerDatabaseWithServerFK) TableName() string { return "docker_databases" }

// docker_deployments (polymorphic across applications + composes) ------------

type dockerDeploymentMigration struct {
	ID         string     `gorm:"type:char(26);primaryKey"`
	TeamID     string     `gorm:"column:team_id;type:char(26);not null;index"`
	ServerID   string     `gorm:"column:server_id;type:char(26);not null;index"`
	TargetType string     `gorm:"column:target_type;type:varchar(32);not null;index:idx_docker_deployments_target"`
	TargetID   string     `gorm:"column:target_id;type:char(26);not null;index:idx_docker_deployments_target"`
	Status     string     `gorm:"type:varchar(32);not null;default:pending"`
	CommitSHA  *string    `gorm:"column:commit_sha;type:varchar(64)"`
	CommitMsg  *string    `gorm:"column:commit_msg;type:text"`
	ImageRef   *string    `gorm:"column:image_ref;type:varchar(512)"`
	LogPath    *string    `gorm:"column:log_path;type:varchar(512)"`
	StartedAt  *time.Time `gorm:"column:started_at;type:timestamp null"`
	FinishedAt *time.Time `gorm:"column:finished_at;type:timestamp null"`
	Error      *string    `gorm:"type:text"`
	CreatedAt  *time.Time `gorm:"type:timestamp null"`
	UpdatedAt  *time.Time `gorm:"type:timestamp null"`
}

func (dockerDeploymentMigration) TableName() string { return "docker_deployments" }

// Child tables: env vars, domains, volumes, schedules ------------------------

type dockerApplicationEnvVarMigration struct {
	ID            string         `gorm:"type:char(26);primaryKey"`
	ApplicationID string         `gorm:"column:application_id;type:char(26);not null;index"`
	Key           string         `gorm:"type:varchar(255);not null"`
	Value         string         `gorm:"type:longtext;not null"`
	IsSecret      bool           `gorm:"column:is_secret;not null;default:false"`
	CreatedAt     *time.Time     `gorm:"type:timestamp null"`
	UpdatedAt     *time.Time     `gorm:"type:timestamp null"`
	DeletedAt     gorm.DeletedAt `gorm:"index"`
}

func (dockerApplicationEnvVarMigration) TableName() string { return "docker_application_env_vars" }

type dockerApplicationEnvVarWithAppFK struct {
	ApplicationID string                      `gorm:"column:application_id"`
	Application   *dockerApplicationMigration `gorm:"foreignKey:ApplicationID;references:ID;constraint:OnDelete:CASCADE"`
}

func (dockerApplicationEnvVarWithAppFK) TableName() string { return "docker_application_env_vars" }

type dockerApplicationDomainMigration struct {
	ID            string         `gorm:"type:char(26);primaryKey"`
	ApplicationID string         `gorm:"column:application_id;type:char(26);not null;index"`
	Host          string         `gorm:"type:varchar(255);not null"`
	Path          *string        `gorm:"type:varchar(255)"`
	HTTPS         bool           `gorm:"column:https;not null;default:true"`
	CertificateID *string        `gorm:"column:certificate_id;type:char(26)"`
	CreatedAt     *time.Time     `gorm:"type:timestamp null"`
	UpdatedAt     *time.Time     `gorm:"type:timestamp null"`
	DeletedAt     gorm.DeletedAt `gorm:"index"`
}

func (dockerApplicationDomainMigration) TableName() string { return "docker_application_domains" }

type dockerApplicationDomainWithAppFK struct {
	ApplicationID string                      `gorm:"column:application_id"`
	Application   *dockerApplicationMigration `gorm:"foreignKey:ApplicationID;references:ID;constraint:OnDelete:CASCADE"`
}

func (dockerApplicationDomainWithAppFK) TableName() string { return "docker_application_domains" }

type dockerApplicationVolumeMigration struct {
	ID            string         `gorm:"type:char(26);primaryKey"`
	ApplicationID string         `gorm:"column:application_id;type:char(26);not null;index"`
	Name          string         `gorm:"type:varchar(255);not null"`
	MountPath     string         `gorm:"column:mount_path;type:varchar(512);not null"`
	Type          string         `gorm:"type:varchar(32);not null"`
	HostPath      *string        `gorm:"column:host_path;type:varchar(512)"`
	CreatedAt     *time.Time     `gorm:"type:timestamp null"`
	UpdatedAt     *time.Time     `gorm:"type:timestamp null"`
	DeletedAt     gorm.DeletedAt `gorm:"index"`
}

func (dockerApplicationVolumeMigration) TableName() string { return "docker_application_volumes" }

type dockerApplicationVolumeWithAppFK struct {
	ApplicationID string                      `gorm:"column:application_id"`
	Application   *dockerApplicationMigration `gorm:"foreignKey:ApplicationID;references:ID;constraint:OnDelete:CASCADE"`
}

func (dockerApplicationVolumeWithAppFK) TableName() string { return "docker_application_volumes" }

type dockerApplicationScheduleMigration struct {
	ID            string         `gorm:"type:char(26);primaryKey"`
	ApplicationID string         `gorm:"column:application_id;type:char(26);not null;index"`
	Cron          string         `gorm:"type:varchar(255);not null"`
	Command       string         `gorm:"type:text;not null"`
	LastRunAt     *time.Time     `gorm:"column:last_run_at;type:timestamp null"`
	LastStatus    *string        `gorm:"column:last_status;type:varchar(32)"`
	CreatedAt     *time.Time     `gorm:"type:timestamp null"`
	UpdatedAt     *time.Time     `gorm:"type:timestamp null"`
	DeletedAt     gorm.DeletedAt `gorm:"index"`
}

func (dockerApplicationScheduleMigration) TableName() string {
	return "docker_application_schedules"
}

type dockerApplicationScheduleWithAppFK struct {
	ApplicationID string                      `gorm:"column:application_id"`
	Application   *dockerApplicationMigration `gorm:"foreignKey:ApplicationID;references:ID;constraint:OnDelete:CASCADE"`
}

func (dockerApplicationScheduleWithAppFK) TableName() string {
	return "docker_application_schedules"
}

func createDockerWorkloadsTablesUp(db *gorm.DB) error {
	migrator := db.Migrator()

	// Order matters — children depend on parents (FK constraints).
	tables := []any{
		&dockerApplicationMigration{},
		&dockerComposeMigration{},
		&dockerDatabaseMigration{},
		&dockerDeploymentMigration{},
		&dockerApplicationEnvVarMigration{},
		&dockerApplicationDomainMigration{},
		&dockerApplicationVolumeMigration{},
		&dockerApplicationScheduleMigration{},
	}
	for _, t := range tables {
		if err := migrator.CreateTable(t); err != nil {
			return err
		}
	}

	type fk struct {
		dst    any
		assoc  string
		uniqOn string
	}
	fks := []fk{
		{&dockerApplicationWithProjectFK{}, "Project", "project_id, name"},
		{&dockerApplicationWithServerFK{}, "Server", ""},
		{&dockerComposeWithProjectFK{}, "Project", "project_id, name"},
		{&dockerComposeWithServerFK{}, "Server", ""},
		{&dockerDatabaseWithProjectFK{}, "Project", "project_id, name"},
		{&dockerDatabaseWithServerFK{}, "Server", ""},
		{&dockerApplicationEnvVarWithAppFK{}, "Application", "application_id, `key`"},
		{&dockerApplicationDomainWithAppFK{}, "Application", "application_id, host"},
		{&dockerApplicationVolumeWithAppFK{}, "Application", "application_id, name"},
		{&dockerApplicationScheduleWithAppFK{}, "Application", ""},
	}
	for _, f := range fks {
		if err := migrator.CreateConstraint(f.dst, f.assoc); err != nil {
			return err
		}
	}

	// Per-parent uniqueness of names. Includes deleted_at in the index so
	// soft-deleting an application doesn't block creating one with the same
	// name (same rationale as in the projects migration).
	uniques := []struct {
		name  string
		table string
		cols  string
	}{
		{"idx_docker_apps_project_name", "docker_applications", "project_id, name, deleted_at"},
		{"idx_docker_composes_project_name", "docker_composes", "project_id, name, deleted_at"},
		{"idx_docker_databases_project_name", "docker_databases", "project_id, name, deleted_at"},
		{"idx_docker_app_env_app_key", "docker_application_env_vars", "application_id, `key`, deleted_at"},
		{"idx_docker_app_domain_app_host", "docker_application_domains", "application_id, host, deleted_at"},
		{"idx_docker_app_volume_app_name", "docker_application_volumes", "application_id, name, deleted_at"},
	}
	for _, u := range uniques {
		if err := db.Exec(
			"CREATE UNIQUE INDEX " + u.name + " ON " + u.table + " (" + u.cols + ")",
		).Error; err != nil {
			return err
		}
	}

	return nil
}

func createDockerWorkloadsTablesDown(db *gorm.DB) error {
	// Drop in reverse order so child FK constraints don't block.
	tables := []any{
		&dockerApplicationScheduleMigration{},
		&dockerApplicationVolumeMigration{},
		&dockerApplicationDomainMigration{},
		&dockerApplicationEnvVarMigration{},
		&dockerDeploymentMigration{},
		&dockerDatabaseMigration{},
		&dockerComposeMigration{},
		&dockerApplicationMigration{},
	}
	for _, t := range tables {
		if err := db.Migrator().DropTable(t); err != nil {
			return err
		}
	}
	return nil
}
