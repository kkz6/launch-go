package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0022_04_27_000000_create_docker_apps_tables",
		Name:      "Create docker apps tables",
		Timestamp: time.Date(2026, 4, 27, 0, 0, 1, 0, time.UTC),
		Up:        createDockerAppsTablesUp,
		Down:      createDockerAppsTablesDown,
	})
}

type dockerAppMigration struct {
	ID       string `gorm:"type:char(26);primaryKey"`
	TeamID   string `gorm:"column:team_id;type:char(26);not null;index"`
	ServerID string `gorm:"column:server_id;type:char(26);not null;index"`

	Name   string `gorm:"type:varchar(64);not null;index"`
	Source string `gorm:"type:varchar(16);not null"`

	Image string `gorm:"type:varchar(255);not null"`
	Tag   string `gorm:"type:varchar(255);not null;default:latest"`

	RegistryCredentialID *string `gorm:"column:registry_credential_id;type:char(26);index"`

	RestartPolicy string `gorm:"type:varchar(32);not null;default:unless-stopped"`
	Status        string `gorm:"type:varchar(32);not null;index"`

	LastError  *string    `gorm:"column:last_error;type:text"`
	TaskID     *string    `gorm:"column:task_id;type:char(26);index"`
	DeployedAt *time.Time `gorm:"column:deployed_at;type:timestamp null"`

	CreatedAt *time.Time `gorm:"type:timestamp null"`
	UpdatedAt *time.Time `gorm:"type:timestamp null"`
}

func (dockerAppMigration) TableName() string { return "docker_apps" }

type dockerAppWithTeamFK struct {
	TeamID string         `gorm:"column:team_id"`
	Team   *teamMigration `gorm:"foreignKey:TeamID;references:ID;constraint:OnDelete:CASCADE"`
}

func (dockerAppWithTeamFK) TableName() string { return "docker_apps" }

type dockerAppWithServerFK struct {
	ServerID string           `gorm:"column:server_id"`
	Server   *serverMigration `gorm:"foreignKey:ServerID;references:ID;constraint:OnDelete:CASCADE"`
}

func (dockerAppWithServerFK) TableName() string { return "docker_apps" }

// Sub-resources --------------------------------------------------------------

type dockerAppEnvVarMigration struct {
	ID     string `gorm:"type:char(26);primaryKey"`
	AppID  string `gorm:"column:app_id;type:char(26);not null;index"`
	Key    string `gorm:"type:varchar(255);not null"`
	Value  string `gorm:"type:longtext;not null"`
	Secret bool   `gorm:"type:tinyint(1);not null;default:0"`

	CreatedAt *time.Time `gorm:"type:timestamp null"`
	UpdatedAt *time.Time `gorm:"type:timestamp null"`
}

func (dockerAppEnvVarMigration) TableName() string { return "docker_app_env_vars" }

type dockerAppEnvVarWithAppFK struct {
	AppID string              `gorm:"column:app_id"`
	App   *dockerAppMigration `gorm:"foreignKey:AppID;references:ID;constraint:OnDelete:CASCADE"`
}

func (dockerAppEnvVarWithAppFK) TableName() string { return "docker_app_env_vars" }

type dockerAppPortMigration struct {
	ID            string `gorm:"type:char(26);primaryKey"`
	AppID         string `gorm:"column:app_id;type:char(26);not null;index"`
	HostPort      int    `gorm:"column:host_port;type:int;not null"`
	ContainerPort int    `gorm:"column:container_port;type:int;not null"`
	Protocol      string `gorm:"type:varchar(8);not null;default:tcp"`

	CreatedAt *time.Time `gorm:"type:timestamp null"`
	UpdatedAt *time.Time `gorm:"type:timestamp null"`
}

func (dockerAppPortMigration) TableName() string { return "docker_app_ports" }

type dockerAppPortWithAppFK struct {
	AppID string              `gorm:"column:app_id"`
	App   *dockerAppMigration `gorm:"foreignKey:AppID;references:ID;constraint:OnDelete:CASCADE"`
}

func (dockerAppPortWithAppFK) TableName() string { return "docker_app_ports" }

type dockerAppVolumeMigration struct {
	ID        string `gorm:"type:char(26);primaryKey"`
	AppID     string `gorm:"column:app_id;type:char(26);not null;index"`
	Name      string `gorm:"type:varchar(64);not null"`
	MountPath string `gorm:"column:mount_path;type:varchar(255);not null"`

	CreatedAt *time.Time `gorm:"type:timestamp null"`
	UpdatedAt *time.Time `gorm:"type:timestamp null"`
}

func (dockerAppVolumeMigration) TableName() string { return "docker_app_volumes" }

type dockerAppVolumeWithAppFK struct {
	AppID string              `gorm:"column:app_id"`
	App   *dockerAppMigration `gorm:"foreignKey:AppID;references:ID;constraint:OnDelete:CASCADE"`
}

func (dockerAppVolumeWithAppFK) TableName() string { return "docker_app_volumes" }

type dockerAppDomainMigration struct {
	ID            string `gorm:"type:char(26);primaryKey"`
	AppID         string `gorm:"column:app_id;type:char(26);not null;index"`
	Domain        string `gorm:"type:varchar(255);not null;index"`
	ContainerPort int    `gorm:"column:container_port;type:int;not null"`
	TLS           bool   `gorm:"type:tinyint(1);not null;default:1"`

	CreatedAt *time.Time `gorm:"type:timestamp null"`
	UpdatedAt *time.Time `gorm:"type:timestamp null"`
}

func (dockerAppDomainMigration) TableName() string { return "docker_app_domains" }

type dockerAppDomainWithAppFK struct {
	AppID string              `gorm:"column:app_id"`
	App   *dockerAppMigration `gorm:"foreignKey:AppID;references:ID;constraint:OnDelete:CASCADE"`
}

func (dockerAppDomainWithAppFK) TableName() string { return "docker_app_domains" }

// Up/Down --------------------------------------------------------------------

func createDockerAppsTablesUp(db *gorm.DB) error {
	migrator := db.Set("gorm:table_options", "DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci").Migrator()

	if err := migrator.CreateTable(&dockerAppMigration{}); err != nil {
		return err
	}
	if err := migrator.CreateConstraint(&dockerAppWithTeamFK{}, "Team"); err != nil {
		return err
	}
	if err := migrator.CreateConstraint(&dockerAppWithServerFK{}, "Server"); err != nil {
		return err
	}
	// One app per (server, name).
	if err := db.Exec("CREATE UNIQUE INDEX idx_docker_apps_server_name ON docker_apps(server_id, name)").Error; err != nil {
		return err
	}

	if err := migrator.CreateTable(&dockerAppEnvVarMigration{}); err != nil {
		return err
	}
	if err := migrator.CreateConstraint(&dockerAppEnvVarWithAppFK{}, "App"); err != nil {
		return err
	}
	if err := db.Exec("CREATE UNIQUE INDEX idx_docker_app_env_vars_app_key ON docker_app_env_vars(app_id, `key`)").Error; err != nil {
		return err
	}

	if err := migrator.CreateTable(&dockerAppPortMigration{}); err != nil {
		return err
	}
	if err := migrator.CreateConstraint(&dockerAppPortWithAppFK{}, "App"); err != nil {
		return err
	}

	if err := migrator.CreateTable(&dockerAppVolumeMigration{}); err != nil {
		return err
	}
	if err := migrator.CreateConstraint(&dockerAppVolumeWithAppFK{}, "App"); err != nil {
		return err
	}
	if err := db.Exec("CREATE UNIQUE INDEX idx_docker_app_volumes_app_name ON docker_app_volumes(app_id, name)").Error; err != nil {
		return err
	}

	if err := migrator.CreateTable(&dockerAppDomainMigration{}); err != nil {
		return err
	}
	if err := migrator.CreateConstraint(&dockerAppDomainWithAppFK{}, "App"); err != nil {
		return err
	}
	return db.Exec("CREATE UNIQUE INDEX idx_docker_app_domains_domain ON docker_app_domains(domain)").Error
}

func createDockerAppsTablesDown(db *gorm.DB) error {
	migrator := db.Migrator()
	for _, t := range []any{
		&dockerAppDomainMigration{},
		&dockerAppVolumeMigration{},
		&dockerAppPortMigration{},
		&dockerAppEnvVarMigration{},
		&dockerAppMigration{},
	} {
		if err := migrator.DropTable(t); err != nil {
			return err
		}
	}
	return nil
}
