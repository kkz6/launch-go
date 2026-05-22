package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0021_05_22_000000_create_docker_projects_table",
		Name:      "Create docker_projects table",
		Timestamp: time.Date(2021, 5, 22, 0, 0, 0, 0, time.UTC),
		Up:        createDockerProjectsTableUp,
		Down:      createDockerProjectsTableDown,
	})
}

type dockerProjectMigration struct {
	ID          string         `gorm:"type:char(26);primaryKey"`
	TeamID      string         `gorm:"column:team_id;type:char(26);not null;index"`
	ServerID    string         `gorm:"column:server_id;type:char(26);not null;index"`
	Name        string         `gorm:"type:varchar(255);not null"`
	Description *string        `gorm:"type:varchar(500)"`
	CreatedAt   *time.Time     `gorm:"type:timestamp null"`
	UpdatedAt   *time.Time     `gorm:"type:timestamp null"`
	DeletedAt   gorm.DeletedAt `gorm:"index"`
}

func (dockerProjectMigration) TableName() string { return "docker_projects" }

type dockerProjectWithTeamFK struct {
	TeamID string         `gorm:"column:team_id"`
	Team   *teamMigration `gorm:"foreignKey:TeamID;references:ID;constraint:OnDelete:CASCADE"`
}

func (dockerProjectWithTeamFK) TableName() string { return "docker_projects" }

type dockerProjectWithServerFK struct {
	ServerID string           `gorm:"column:server_id"`
	Server   *serverMigration `gorm:"foreignKey:ServerID;references:ID;constraint:OnDelete:CASCADE"`
}

func (dockerProjectWithServerFK) TableName() string { return "docker_projects" }

func createDockerProjectsTableUp(db *gorm.DB) error {
	// Match the charset/collation used by every other table in this schema
	// (utf8mb4_unicode_ci). MySQL 8 defaults to utf8mb4_0900_ai_ci which is
	// FK-incompatible with the legacy tables — see the older migrations
	// (load_balancer, notification_preferences) for the same incantation.
	migrator := db.Set(
		"gorm:table_options",
		"DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci",
	).Migrator()

	if err := migrator.CreateTable(&dockerProjectMigration{}); err != nil {
		return err
	}

	if err := migrator.CreateConstraint(&dockerProjectWithTeamFK{}, "Team"); err != nil {
		return err
	}

	if err := migrator.CreateConstraint(&dockerProjectWithServerFK{}, "Server"); err != nil {
		return err
	}

	// Unique per-server project name among non-deleted rows. MySQL doesn't
	// support partial indexes, so we lean on the deleted_at column being
	// NULL for live rows and accept that the same name can re-appear after
	// soft-delete (which is the behaviour we want — soft-deleted projects
	// shouldn't block new ones with the same name).
	return db.Exec(
		"CREATE UNIQUE INDEX idx_docker_projects_server_name " +
			"ON docker_projects (server_id, name, deleted_at)",
	).Error
}

func createDockerProjectsTableDown(db *gorm.DB) error {
	return db.Migrator().DropTable(&dockerProjectMigration{})
}
