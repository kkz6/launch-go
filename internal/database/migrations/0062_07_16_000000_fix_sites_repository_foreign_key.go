package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0062_07_16_000000_fix_sites_repository_foreign_key",
		Name:      "Fix sites repository foreign key to set null on delete",
		Timestamp: time.Date(2026, 7, 16, 0, 0, 0, 0, time.UTC),
		Up:        fixSitesRepositoryForeignKeyUp,
	})
}

func fixSitesRepositoryForeignKeyUp(db *gorm.DB) error {
	if err := db.Exec(
		"ALTER TABLE sites DROP CONSTRAINT IF EXISTS sites_source_control_repositories_id_foreign",
	).Error; err != nil {
		return err
	}

	return db.Exec(
		"ALTER TABLE sites ADD CONSTRAINT sites_source_control_repositories_id_foreign " +
			"FOREIGN KEY (source_control_repositories_id) REFERENCES source_control_repositories(id) ON DELETE SET NULL",
	).Error
}
