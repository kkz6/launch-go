package migrations

import (
	"time"

	"gorm.io/gorm"
)

// Adds servers.host_key — the SSH host key (public, base64-encoded
// wire format) the agent pinned the first time it connected to the
// box. Subsequent connections verify the live host key against this
// stored value. NULL means "not pinned yet, the next connection
// will TOFU it in" — same effect as the previous behaviour, just
// with a chance to detect a mismatch on every connection after the
// first.
//
// Host keys are public so this column is plain text, not encrypted.
// The threat model is integrity (detect MITM / IP reuse) not
// confidentiality.
func init() {
	Register(Migration{
		ID:        "0055_06_01_000000_add_host_key_to_servers",
		Name:      "Add host_key column to servers",
		Timestamp: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		Up:        addHostKeyToServersUp,
		Down:      addHostKeyToServersDown,
	})
}

func addHostKeyToServersUp(db *gorm.DB) error {
	return db.Exec(`ALTER TABLE servers ADD COLUMN IF NOT EXISTS host_key TEXT NULL`).Error
}

func addHostKeyToServersDown(db *gorm.DB) error {
	return db.Exec(`ALTER TABLE servers DROP COLUMN IF EXISTS host_key`).Error
}
