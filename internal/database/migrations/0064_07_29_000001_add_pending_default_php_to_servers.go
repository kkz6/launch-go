package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0064_07_29_000001_add_pending_default_php_to_servers",
		Name:      "Add pending default PHP service to servers",
		Timestamp: time.Date(2026, 7, 29, 0, 0, 1, 0, time.UTC),
		Up:        addPendingDefaultPhpToServersUp,
	})
}

func addPendingDefaultPhpToServersUp(db *gorm.DB) error {
	return db.Exec(
		"ALTER TABLE servers ADD COLUMN IF NOT EXISTS pending_default_php_service_id CHAR(26)",
	).Error
}
