package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0072_11_14_000000_add_backup_source_database",
		Name:      "Record database backup source namespace",
		Timestamp: time.Date(2026, 11, 14, 0, 0, 0, 0, time.UTC),
		Up:        addBackupSourceDatabaseUp,
	})
}

func addBackupSourceDatabaseUp(db *gorm.DB) error {
	if db.Migrator().HasColumn("docker_database_backup_runs", "source_database") {
		return nil
	}
	// Do not backfill from current backup settings: historical source names
	// cannot be safely inferred after those settings have changed.
	return db.Exec("ALTER TABLE docker_database_backup_runs ADD COLUMN source_database VARCHAR(64)").Error
}
