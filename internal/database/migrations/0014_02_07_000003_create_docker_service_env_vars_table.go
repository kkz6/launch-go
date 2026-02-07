package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0014_02_07_000003_create_docker_service_env_vars_table",
		Name:      "Create docker service env vars table",
		Timestamp: time.Date(2014, 2, 7, 0, 0, 3, 0, time.UTC),
		Up:        createDockerServiceEnvVarsTableUp,
		Down:      createDockerServiceEnvVarsTableDown,
	})
}

type dockerServiceEnvVarMigration struct {
	ID              string `gorm:"type:char(26);primaryKey"`
	DockerServiceID string `gorm:"column:docker_service_id;type:char(26);not null;index"`

	Key      string `gorm:"type:varchar(255);not null"`
	Value    string `gorm:"type:text;not null"`
	IsSecret bool   `gorm:"column:is_secret;not null;default:false"`

	CreatedAt *time.Time `gorm:"type:timestamp null"`
	UpdatedAt *time.Time `gorm:"type:timestamp null"`
}

func (dockerServiceEnvVarMigration) TableName() string { return "docker_service_env_vars" }

type dockerServiceEnvVarWithServiceFK struct {
	DockerServiceID string                  `gorm:"column:docker_service_id"`
	DockerService   *dockerServiceMigration `gorm:"foreignKey:DockerServiceID;references:ID;constraint:OnDelete:CASCADE"`
}

func (dockerServiceEnvVarWithServiceFK) TableName() string { return "docker_service_env_vars" }

func createDockerServiceEnvVarsTableUp(db *gorm.DB) error {
	if err := db.Migrator().CreateTable(&dockerServiceEnvVarMigration{}); err != nil {
		return err
	}

	return db.Migrator().CreateConstraint(&dockerServiceEnvVarWithServiceFK{}, "DockerService")
}

func createDockerServiceEnvVarsTableDown(db *gorm.DB) error {
	return db.Migrator().DropTable(&dockerServiceEnvVarMigration{})
}
