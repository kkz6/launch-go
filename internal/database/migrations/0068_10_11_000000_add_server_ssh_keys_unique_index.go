package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0068_10_11_000000_add_server_ssh_keys_unique_index",
		Name:      "Ensure server SSH key attachments are unique",
		Timestamp: time.Date(2026, 10, 11, 0, 0, 0, 0, time.UTC),
		Up:        addServerSSHKeysUniqueIndexUp,
	})
}

// Legacy MySQL imports retained only the individual foreign-key indexes,
// unlike fresh installations with a composite primary key. PostgreSQL needs
// a unique index on this pair to support the attachment ON CONFLICT clause.
func addServerSSHKeysUniqueIndexUp(db *gorm.DB) error {
	return db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_server_ssh_keys_unique_attachment
		ON server_ssh_keys (server_id, ssh_key_id)`).Error
}
