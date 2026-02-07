package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0014_02_07_000001_create_docker_compose_projects_table",
		Name:      "Create docker compose projects table",
		Timestamp: time.Date(2014, 2, 7, 0, 0, 1, 0, time.UTC),
		Up:        createDockerComposeProjectsTableUp,
		Down:      createDockerComposeProjectsTableDown,
	})
}

type dockerComposeProjectMigration struct {
	ID       string `gorm:"type:char(26);primaryKey"`
	TeamID   string `gorm:"column:team_id;type:char(26);not null;index"`
	ServerID string `gorm:"column:server_id;type:char(26);not null;index"`

	Name       string `gorm:"type:varchar(255);not null"`
	RawCompose string `gorm:"column:raw_compose;type:text;not null"`

	CreatedAt *time.Time `gorm:"type:timestamp null"`
	UpdatedAt *time.Time `gorm:"type:timestamp null"`
}

func (dockerComposeProjectMigration) TableName() string { return "docker_compose_projects" }

type dockerComposeProjectWithTeamFK struct {
	TeamID string         `gorm:"column:team_id"`
	Team   *teamMigration `gorm:"foreignKey:TeamID;references:ID;constraint:OnDelete:CASCADE"`
}

func (dockerComposeProjectWithTeamFK) TableName() string { return "docker_compose_projects" }

type dockerComposeProjectWithServerFK struct {
	ServerID string           `gorm:"column:server_id"`
	Server   *serverMigration `gorm:"foreignKey:ServerID;references:ID;constraint:OnDelete:CASCADE"`
}

func (dockerComposeProjectWithServerFK) TableName() string { return "docker_compose_projects" }

func createDockerComposeProjectsTableUp(db *gorm.DB) error {
	migrator := db.Migrator()

	if err := migrator.CreateTable(&dockerComposeProjectMigration{}); err != nil {
		return err
	}

	if err := migrator.CreateConstraint(&dockerComposeProjectWithTeamFK{}, "Team"); err != nil {
		return err
	}

	return migrator.CreateConstraint(&dockerComposeProjectWithServerFK{}, "Server")
}

func createDockerComposeProjectsTableDown(db *gorm.DB) error {
	return db.Migrator().DropTable(&dockerComposeProjectMigration{})
}
