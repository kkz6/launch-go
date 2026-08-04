package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0063_07_29_000000_add_pending_php_version_to_sites",
		Name:      "Add pending_php_version to sites",
		Timestamp: time.Date(2026, 7, 29, 0, 0, 0, 0, time.UTC),
		Up:        addPendingPhpVersionToSitesUp,
	})
}

func addPendingPhpVersionToSitesUp(db *gorm.DB) error {
	return db.Exec(
		"ALTER TABLE sites ADD COLUMN IF NOT EXISTS pending_php_version VARCHAR(255)",
	).Error
}
