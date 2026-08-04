package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0065_07_30_000000_add_tls_rollback_state_to_sites",
		Name:      "Add TLS rollback state to sites",
		Timestamp: time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC),
		Up:        addTLSRollbackStateToSitesUp,
	})
}

func addTLSRollbackStateToSitesUp(db *gorm.DB) error {
	statements := []string{
		"ALTER TABLE sites ADD COLUMN IF NOT EXISTS pending_tls_previous_setting VARCHAR(255)",
		"ALTER TABLE sites ADD COLUMN IF NOT EXISTS pending_tls_previous_certificate_ids JSON",
		"ALTER TABLE sites ADD COLUMN IF NOT EXISTS pending_tls_replacement_certificate_id CHAR(26)",
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			return err
		}
	}
	return nil
}
