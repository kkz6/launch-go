package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0073_11_16_000001_create_server_restarts",
		Name:      "Track server restart outcomes",
		Timestamp: time.Date(2026, 11, 16, 0, 0, 1, 0, time.UTC),
		Up: func(db *gorm.DB) error {
			return db.Exec(`CREATE TABLE IF NOT EXISTS server_restarts (
				server_id CHAR(26) PRIMARY KEY REFERENCES servers(id) ON DELETE CASCADE,
				operation_id CHAR(26) NOT NULL DEFAULT '',
				status VARCHAR(20) NOT NULL DEFAULT 'idle',
				boot_id VARCHAR(36) NOT NULL DEFAULT '',
				requested_at TIMESTAMPTZ NULL,
				completed_at TIMESTAMPTZ NULL,
				deadline_at TIMESTAMPTZ NULL,
				check_after TIMESTAMPTZ NULL,
				message TEXT NOT NULL DEFAULT ''
			)`).Error
		},
	})
}
