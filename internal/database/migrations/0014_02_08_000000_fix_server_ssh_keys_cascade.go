package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0014_02_08_000000_fix_server_ssh_keys_cascade",
		Name:      "Fix server_ssh_keys foreign keys to cascade on delete",
		Timestamp: time.Date(2014, 2, 8, 0, 0, 0, 0, time.UTC),
		Up:        fixServerSSHKeysCascadeUp,
	})
}

func fixServerSSHKeysCascadeUp(db *gorm.DB) error {
	// Drop existing constraints
	if err := db.Exec(
		"ALTER TABLE server_ssh_keys DROP CONSTRAINT IF EXISTS server_ssh_keys_server_id_foreign",
	).Error; err != nil {
		return err
	}

	if err := db.Exec(
		"ALTER TABLE server_ssh_keys DROP CONSTRAINT IF EXISTS server_ssh_keys_ssh_key_id_foreign",
	).Error; err != nil {
		return err
	}

	// Recreate with ON DELETE CASCADE
	if err := db.Exec(
		"ALTER TABLE server_ssh_keys ADD CONSTRAINT server_ssh_keys_server_id_foreign FOREIGN KEY (server_id) REFERENCES servers(id) ON DELETE CASCADE",
	).Error; err != nil {
		return err
	}

	return db.Exec(
		"ALTER TABLE server_ssh_keys ADD CONSTRAINT server_ssh_keys_ssh_key_id_foreign FOREIGN KEY (ssh_key_id) REFERENCES ssh_keys(id) ON DELETE CASCADE",
	).Error
}
