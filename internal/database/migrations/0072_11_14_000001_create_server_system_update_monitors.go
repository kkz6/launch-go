package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0072_11_14_000001_create_server_system_update_monitors",
		Name:      "Track system update notifications",
		Timestamp: time.Date(2026, 11, 14, 0, 0, 1, 0, time.UTC),
		Up: func(db *gorm.DB) error {
			return db.Exec(`CREATE TABLE IF NOT EXISTS server_system_update_monitors (
				server_id CHAR(26) PRIMARY KEY REFERENCES servers(id) ON DELETE CASCADE,
				last_run_id VARCHAR(40) NOT NULL DEFAULT '',
				reboot_id VARCHAR(40) NOT NULL DEFAULT '',
				state_hash VARCHAR(64) NOT NULL DEFAULT ''
			)`).Error
		},
	})
}
