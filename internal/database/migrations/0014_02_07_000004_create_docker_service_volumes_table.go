package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0014_02_07_000004_create_docker_service_volumes_table",
		Name:      "Create docker service volumes table",
		Timestamp: time.Date(2014, 2, 7, 0, 0, 4, 0, time.UTC),
		Up:        createDockerServiceVolumesTableUp,
		Down:      createDockerServiceVolumesTableDown,
	})
}

type dockerServiceVolumeMigration struct {
	ID              string `gorm:"type:char(26);primaryKey"`
	DockerServiceID string `gorm:"column:docker_service_id;type:char(26);not null;index"`

	MountType string `gorm:"column:mount_type;type:varchar(50);not null;default:'volume'"`
	Source    string `gorm:"type:varchar(500);not null"`
	Target    string `gorm:"type:varchar(500);not null"`
	ReadOnly  bool   `gorm:"column:read_only;not null;default:false"`

	CreatedAt *time.Time `gorm:"type:timestamp null"`
	UpdatedAt *time.Time `gorm:"type:timestamp null"`
}

func (dockerServiceVolumeMigration) TableName() string { return "docker_service_volumes" }

type dockerServiceVolumeWithServiceFK struct {
	DockerServiceID string                  `gorm:"column:docker_service_id"`
	DockerService   *dockerServiceMigration `gorm:"foreignKey:DockerServiceID;references:ID;constraint:OnDelete:CASCADE"`
}

func (dockerServiceVolumeWithServiceFK) TableName() string { return "docker_service_volumes" }

func createDockerServiceVolumesTableUp(db *gorm.DB) error {
	if err := db.Migrator().CreateTable(&dockerServiceVolumeMigration{}); err != nil {
		return err
	}

	return db.Migrator().CreateConstraint(&dockerServiceVolumeWithServiceFK{}, "DockerService")
}

func createDockerServiceVolumesTableDown(db *gorm.DB) error {
	return db.Migrator().DropTable(&dockerServiceVolumeMigration{})
}
