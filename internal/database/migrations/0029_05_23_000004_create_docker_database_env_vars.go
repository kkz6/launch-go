package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0029_05_23_000004_create_docker_database_env_vars",
		Name:      "Create docker_database_env_vars table",
		Timestamp: time.Date(2024, 5, 23, 0, 0, 4, 0, time.UTC),
		Up:        createDockerDatabaseEnvVarsUp,
		Down:      createDockerDatabaseEnvVarsDown,
	})
}

// docker_database_env_vars — additional env vars the user can add to a
// managed database container on top of the auto-generated engine
// credentials (POSTGRES_USER, MYSQL_ROOT_PASSWORD, etc). Common uses:
// per-engine tuning flags (POSTGRES_INITDB_ARGS, MYSQL_DEFAULT_AUTH...).
//
// Values may reference project env vars via ${{project.<KEY>}}; the
// run script resolver substitutes those at docker-run time.
type dockerDatabaseEnvVarMigration struct {
	ID         string         `gorm:"type:char(26);primaryKey"`
	DatabaseID string         `gorm:"column:database_id;type:char(26);not null;index"`
	Key        string         `gorm:"type:varchar(255);not null"`
	Value      string         `gorm:"type:text;not null"`
	IsSecret   bool           `gorm:"column:is_secret;not null;default:false"`
	CreatedAt  *time.Time     `gorm:"type:timestamp null"`
	UpdatedAt  *time.Time     `gorm:"type:timestamp null"`
	DeletedAt  gorm.DeletedAt `gorm:"index"`
}

func (dockerDatabaseEnvVarMigration) TableName() string {
	return "docker_database_env_vars"
}

type dockerDatabaseEnvVarWithDatabaseFK struct {
	DatabaseID string                   `gorm:"column:database_id"`
	Database   *dockerDatabaseMigration `gorm:"foreignKey:DatabaseID;references:ID;constraint:OnDelete:CASCADE"`
}

func (dockerDatabaseEnvVarWithDatabaseFK) TableName() string {
	return "docker_database_env_vars"
}

func createDockerDatabaseEnvVarsUp(db *gorm.DB) error {
	migrator := db.Migrator()

	if err := migrator.CreateTable(&dockerDatabaseEnvVarMigration{}); err != nil {
		return err
	}
	if err := migrator.CreateConstraint(&dockerDatabaseEnvVarWithDatabaseFK{}, "Database"); err != nil {
		return err
	}
	return db.Exec(
		"CREATE UNIQUE INDEX idx_docker_database_env_vars_unique " +
			"ON docker_database_env_vars (database_id, key, deleted_at)",
	).Error
}

func createDockerDatabaseEnvVarsDown(db *gorm.DB) error {
	return db.Migrator().DropTable(&dockerDatabaseEnvVarMigration{})
}
