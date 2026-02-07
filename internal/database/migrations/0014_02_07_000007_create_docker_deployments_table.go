package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0014_02_07_000007_create_docker_deployments_table",
		Name:      "Create docker deployments table",
		Timestamp: time.Date(2014, 2, 7, 0, 0, 7, 0, time.UTC),
		Up:        createDockerDeploymentsTableUp,
		Down:      createDockerDeploymentsTableDown,
	})
}

type dockerDeploymentMigration struct {
	ID              string `gorm:"type:char(26);primaryKey"`
	DockerServiceID string `gorm:"column:docker_service_id;type:char(26);not null;index"`

	Image   string  `gorm:"type:varchar(500);not null"`
	Status  string  `gorm:"type:varchar(50);not null;default:'pending'"`
	Trigger string  `gorm:"type:varchar(50);not null;default:'manual'"`
	Log     *string `gorm:"type:text"`

	StartedAt  *time.Time `gorm:"column:started_at;type:timestamp null"`
	FinishedAt *time.Time `gorm:"column:finished_at;type:timestamp null"`

	CreatedAt *time.Time `gorm:"type:timestamp null"`
	UpdatedAt *time.Time `gorm:"type:timestamp null"`
}

func (dockerDeploymentMigration) TableName() string { return "docker_deployments" }

type dockerDeploymentWithServiceFK struct {
	DockerServiceID string                  `gorm:"column:docker_service_id"`
	DockerService   *dockerServiceMigration `gorm:"foreignKey:DockerServiceID;references:ID;constraint:OnDelete:CASCADE"`
}

func (dockerDeploymentWithServiceFK) TableName() string { return "docker_deployments" }

func createDockerDeploymentsTableUp(db *gorm.DB) error {
	if err := db.Migrator().CreateTable(&dockerDeploymentMigration{}); err != nil {
		return err
	}

	return db.Migrator().CreateConstraint(&dockerDeploymentWithServiceFK{}, "DockerService")
}

func createDockerDeploymentsTableDown(db *gorm.DB) error {
	return db.Migrator().DropTable(&dockerDeploymentMigration{})
}
