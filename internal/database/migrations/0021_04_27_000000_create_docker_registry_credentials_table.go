package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0021_04_27_000000_create_docker_registry_credentials_table",
		Name:      "Create docker registry credentials table",
		Timestamp: time.Date(2026, 4, 27, 0, 0, 0, 0, time.UTC),
		Up:        createDockerRegistryCredentialsTableUp,
		Down:      createDockerRegistryCredentialsTableDown,
	})
}

type dockerRegistryCredentialMigration struct {
	ID     string `gorm:"type:char(26);primaryKey"`
	TeamID string `gorm:"column:team_id;type:char(26);not null;index"`

	Name string `gorm:"type:varchar(255);not null"`
	Type string `gorm:"type:varchar(32);not null;index"`
	URL  string `gorm:"type:varchar(255);not null"`

	Username string `gorm:"type:longtext;not null"`
	Password string `gorm:"type:longtext;not null"`

	CreatedAt *time.Time `gorm:"type:timestamp null"`
	UpdatedAt *time.Time `gorm:"type:timestamp null"`
}

func (dockerRegistryCredentialMigration) TableName() string {
	return "docker_registry_credentials"
}

type dockerRegistryCredentialWithTeamFK struct {
	TeamID string         `gorm:"column:team_id"`
	Team   *teamMigration `gorm:"foreignKey:TeamID;references:ID;constraint:OnDelete:CASCADE"`
}

func (dockerRegistryCredentialWithTeamFK) TableName() string {
	return "docker_registry_credentials"
}

func createDockerRegistryCredentialsTableUp(db *gorm.DB) error {
	migrator := db.Set("gorm:table_options", "DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci").Migrator()

	if err := migrator.CreateTable(&dockerRegistryCredentialMigration{}); err != nil {
		return err
	}

	if err := migrator.CreateConstraint(&dockerRegistryCredentialWithTeamFK{}, "Team"); err != nil {
		return err
	}

	// Names must be unique within a team so the dropdown stays unambiguous.
	return db.Exec("CREATE UNIQUE INDEX idx_docker_registry_credentials_team_name ON docker_registry_credentials(team_id, name)").Error
}

func createDockerRegistryCredentialsTableDown(db *gorm.DB) error {
	return db.Migrator().DropTable(&dockerRegistryCredentialMigration{})
}
