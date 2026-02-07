package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0014_02_07_000000_create_docker_registries_table",
		Name:      "Create docker registries table",
		Timestamp: time.Date(2014, 2, 7, 0, 0, 0, 0, time.UTC),
		Up:        createDockerRegistriesTableUp,
		Down:      createDockerRegistriesTableDown,
	})
}

type dockerRegistryMigration struct {
	ID     string `gorm:"type:char(26);primaryKey"`
	TeamID string `gorm:"column:team_id;type:char(26);not null;index"`

	Name     string `gorm:"type:varchar(255);not null"`
	URL      string `gorm:"type:varchar(500);not null"`
	Username string `gorm:"type:varchar(255);not null"`
	Password string `gorm:"type:longtext;not null"`

	CreatedAt *time.Time `gorm:"type:timestamp null"`
	UpdatedAt *time.Time `gorm:"type:timestamp null"`
}

func (dockerRegistryMigration) TableName() string { return "docker_registries" }

type dockerRegistryWithTeamFK struct {
	TeamID string         `gorm:"column:team_id"`
	Team   *teamMigration `gorm:"foreignKey:TeamID;references:ID;constraint:OnDelete:CASCADE"`
}

func (dockerRegistryWithTeamFK) TableName() string { return "docker_registries" }

func createDockerRegistriesTableUp(db *gorm.DB) error {
	if err := db.Migrator().CreateTable(&dockerRegistryMigration{}); err != nil {
		return err
	}

	return db.Migrator().CreateConstraint(&dockerRegistryWithTeamFK{}, "Team")
}

func createDockerRegistriesTableDown(db *gorm.DB) error {
	return db.Migrator().DropTable(&dockerRegistryMigration{})
}
