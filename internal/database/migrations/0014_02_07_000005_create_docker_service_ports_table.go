package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0014_02_07_000005_create_docker_service_ports_table",
		Name:      "Create docker service ports table",
		Timestamp: time.Date(2014, 2, 7, 0, 0, 5, 0, time.UTC),
		Up:        createDockerServicePortsTableUp,
		Down:      createDockerServicePortsTableDown,
	})
}

type dockerServicePortMigration struct {
	ID              string `gorm:"type:char(26);primaryKey"`
	DockerServiceID string `gorm:"column:docker_service_id;type:char(26);not null;index"`

	HostPort      int    `gorm:"column:host_port;type:int;not null"`
	ContainerPort int    `gorm:"column:container_port;type:int;not null"`
	Protocol      string `gorm:"type:varchar(10);not null;default:'tcp'"`

	CreatedAt *time.Time `gorm:"type:timestamp null"`
	UpdatedAt *time.Time `gorm:"type:timestamp null"`
}

func (dockerServicePortMigration) TableName() string { return "docker_service_ports" }

type dockerServicePortWithServiceFK struct {
	DockerServiceID string                  `gorm:"column:docker_service_id"`
	DockerService   *dockerServiceMigration `gorm:"foreignKey:DockerServiceID;references:ID;constraint:OnDelete:CASCADE"`
}

func (dockerServicePortWithServiceFK) TableName() string { return "docker_service_ports" }

func createDockerServicePortsTableUp(db *gorm.DB) error {
	if err := db.Migrator().CreateTable(&dockerServicePortMigration{}); err != nil {
		return err
	}

	return db.Migrator().CreateConstraint(&dockerServicePortWithServiceFK{}, "DockerService")
}

func createDockerServicePortsTableDown(db *gorm.DB) error {
	return db.Migrator().DropTable(&dockerServicePortMigration{})
}
