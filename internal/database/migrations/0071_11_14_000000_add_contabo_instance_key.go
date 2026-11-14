package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0071_11_14_000000_add_contabo_instance_key",
		Name:      "Identify imported Contabo instances",
		Timestamp: time.Date(2026, 11, 14, 0, 0, 0, 0, time.UTC),
		Up:        addContaboInstanceKeyUp,
	})
}

func addContaboInstanceKeyUp(db *gorm.DB) error {
	if err := db.Exec("ALTER TABLE servers ADD COLUMN IF NOT EXISTS contabo_instance_key VARCHAR(64)").Error; err != nil {
		return err
	}
	return db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_servers_contabo_import ON servers (team_id, contabo_instance_key)").Error
}
