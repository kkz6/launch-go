package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0038_05_24_000004_add_database_name_to_docker_database_backups",
		Name:      "Add optional database_name override column to docker_database_backups",
		Timestamp: time.Date(2024, 5, 24, 0, 0, 4, 0, time.UTC),
		Up:        addDatabaseNameToDockerDatabaseBackupsUp,
	})
}

// addDatabaseNameToDockerDatabaseBackupsUp lets a backup config target a
// specific database inside the engine rather than the single one the
// docker_databases row was provisioned with.
//
// Why this exists:
//
//	A docker_databases row represents one *engine container* (e.g. one
//	Postgres instance) plus the default database created at provision
//	time. Users routinely run `CREATE DATABASE` inside that engine to
//	add more logical DBs — those were silently invisible to backups.
//	The Create Backup dialog now exposes an optional "Database name"
//	field; empty preserves today's behaviour (back up the row's default
//	DB), non-empty overrides the engine-specific dump command's target.
//
// Nullable + no default — empty / NULL means "use the row's default".
// 64-char width matches the credential-side database column elsewhere
// (Postgres/MySQL identifier limits cluster around 63–64).
func addDatabaseNameToDockerDatabaseBackupsUp(db *gorm.DB) error {
	return db.Exec(
		"ALTER TABLE docker_database_backups " +
			"ADD COLUMN database_name VARCHAR(64) NULL",
	).Error
}
