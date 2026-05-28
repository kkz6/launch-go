package migrations

import (
	"time"

	"gorm.io/gorm"
)

// Adds nullable columns to capture what's *actually* running on a
// customer's box, distinct from what they picked in the dropdown at
// server-create time. The detect_os provision step (runs first after
// connection) populates these by sourcing /etc/os-release and uname,
// so downstream scripts (Docker, PHP, agent install) can branch on
// truth rather than user intent.
//
// All nullable: legacy servers stay null until they're reprovisioned
// or a future backfill job fills them in.
func init() {
	Register(Migration{
		ID:        "0049_05_28_000001_add_detected_os_to_servers",
		Name:      "Add detected_os_* and detected_arch / detected_kernel columns to servers",
		Timestamp: time.Date(2026, 5, 28, 0, 0, 1, 0, time.UTC),
		Up:        addDetectedOSToServersUp,
		Down:      addDetectedOSToServersDown,
	})
}

func addDetectedOSToServersUp(db *gorm.DB) error {
	return db.Exec(`
		ALTER TABLE servers
			ADD COLUMN detected_os_id VARCHAR(64) NULL,
			ADD COLUMN detected_os_version VARCHAR(64) NULL,
			ADD COLUMN detected_os_version_codename VARCHAR(64) NULL,
			ADD COLUMN detected_arch VARCHAR(32) NULL,
			ADD COLUMN detected_kernel VARCHAR(128) NULL,
			ADD COLUMN detected_at TIMESTAMPTZ NULL
	`).Error
}

func addDetectedOSToServersDown(db *gorm.DB) error {
	return db.Exec(`
		ALTER TABLE servers
			DROP COLUMN IF EXISTS detected_os_id,
			DROP COLUMN IF EXISTS detected_os_version,
			DROP COLUMN IF EXISTS detected_os_version_codename,
			DROP COLUMN IF EXISTS detected_arch,
			DROP COLUMN IF EXISTS detected_kernel,
			DROP COLUMN IF EXISTS detected_at
	`).Error
}
