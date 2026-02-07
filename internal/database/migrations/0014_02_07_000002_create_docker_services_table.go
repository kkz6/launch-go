package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0014_02_07_000002_create_docker_services_table",
		Name:      "Create docker services table",
		Timestamp: time.Date(2014, 2, 7, 0, 0, 2, 0, time.UTC),
		Up:        createDockerServicesTableUp,
		Down:      createDockerServicesTableDown,
	})
}

type dockerServiceMigration struct {
	ID       string `gorm:"type:char(26);primaryKey"`
	TeamID   string `gorm:"column:team_id;type:char(26);not null;index"`
	ServerID string `gorm:"column:server_id;type:char(26);not null;index"`
	UserID   string `gorm:"column:user_id;type:char(26);not null;index"`

	Name          string `gorm:"type:varchar(255);not null"`
	ContainerName string `gorm:"column:container_name;type:varchar(255);not null"`
	Kind          string `gorm:"type:varchar(50);not null;default:'application'"`
	Status        string `gorm:"type:varchar(50);not null;default:'pending'"`
	Image         string `gorm:"type:varchar(500);not null"`

	RegistryID *string `gorm:"column:registry_id;type:char(26);index"`

	Command       *string `gorm:"type:text"`
	Entrypoint    *string `gorm:"type:text"`
	RestartPolicy string  `gorm:"column:restart_policy;type:varchar(50);not null;default:'unless-stopped'"`

	CPULimit    *float64 `gorm:"column:cpu_limit;type:decimal(5,2)"`
	MemoryLimit *int     `gorm:"column:memory_limit;type:int"`

	HealthCheckCmd      *string `gorm:"column:health_check_cmd;type:text"`
	HealthCheckInterval int     `gorm:"column:health_check_interval;type:int;default:30"`
	HealthCheckTimeout  int     `gorm:"column:health_check_timeout;type:int;default:10"`
	HealthCheckRetries  int     `gorm:"column:health_check_retries;type:int;default:3"`

	DeployToken string `gorm:"column:deploy_token;type:varchar(64);not null"`

	ComposeProjectID *string `gorm:"column:compose_project_id;type:char(26);index"`

	InstalledAt          *time.Time `gorm:"column:installed_at;type:timestamp null"`
	InstallationFailedAt *time.Time `gorm:"column:installation_failed_at;type:timestamp null"`

	CreatedAt *time.Time `gorm:"type:timestamp null"`
	UpdatedAt *time.Time `gorm:"type:timestamp null"`
}

func (dockerServiceMigration) TableName() string { return "docker_services" }

type dockerServiceWithTeamFK struct {
	TeamID string         `gorm:"column:team_id"`
	Team   *teamMigration `gorm:"foreignKey:TeamID;references:ID;constraint:OnDelete:CASCADE"`
}

func (dockerServiceWithTeamFK) TableName() string { return "docker_services" }

type dockerServiceWithServerFK struct {
	ServerID string           `gorm:"column:server_id"`
	Server   *serverMigration `gorm:"foreignKey:ServerID;references:ID;constraint:OnDelete:CASCADE"`
}

func (dockerServiceWithServerFK) TableName() string { return "docker_services" }

type dockerServiceWithUserFK struct {
	UserID string         `gorm:"column:user_id"`
	User   *userMigration `gorm:"foreignKey:UserID;references:ID;constraint:OnDelete:CASCADE"`
}

func (dockerServiceWithUserFK) TableName() string { return "docker_services" }

type dockerServiceWithRegistryFK struct {
	RegistryID *string                  `gorm:"column:registry_id"`
	Registry   *dockerRegistryMigration `gorm:"foreignKey:RegistryID;references:ID;constraint:OnDelete:SET NULL"`
}

func (dockerServiceWithRegistryFK) TableName() string { return "docker_services" }

type dockerServiceWithComposeProjectFK struct {
	ComposeProjectID *string                        `gorm:"column:compose_project_id"`
	ComposeProject   *dockerComposeProjectMigration `gorm:"foreignKey:ComposeProjectID;references:ID;constraint:OnDelete:SET NULL"`
}

func (dockerServiceWithComposeProjectFK) TableName() string { return "docker_services" }

func createDockerServicesTableUp(db *gorm.DB) error {
	migrator := db.Migrator()

	if err := migrator.CreateTable(&dockerServiceMigration{}); err != nil {
		return err
	}

	if err := migrator.CreateConstraint(&dockerServiceWithTeamFK{}, "Team"); err != nil {
		return err
	}

	if err := migrator.CreateConstraint(&dockerServiceWithServerFK{}, "Server"); err != nil {
		return err
	}

	if err := migrator.CreateConstraint(&dockerServiceWithUserFK{}, "User"); err != nil {
		return err
	}

	if err := migrator.CreateConstraint(&dockerServiceWithRegistryFK{}, "Registry"); err != nil {
		return err
	}

	if err := migrator.CreateConstraint(&dockerServiceWithComposeProjectFK{}, "ComposeProject"); err != nil {
		return err
	}

	return db.Exec("CREATE UNIQUE INDEX idx_docker_services_server_container ON docker_services(server_id, container_name)").Error
}

func createDockerServicesTableDown(db *gorm.DB) error {
	return db.Migrator().DropTable(&dockerServiceMigration{})
}
