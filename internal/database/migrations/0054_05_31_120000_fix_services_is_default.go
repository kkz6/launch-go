package migrations

import (
	"time"

	"gorm.io/gorm"
)

// Two problems with services.is_default that this migration repairs:
//
//  1. The original column DEFAULT was TRUE — so every new service insert
//     that didn't explicitly set is_default became default. For PHP that
//     manifested as "every newly-installed PHP version steals the star
//     from the previous default", leaving multiple rows with
//     is_default=true on the same server.
//
//  2. Existing data: any server with >1 PHP service marked default needs
//     to be cleaned up so the UI star isn't ambiguous. We keep the
//     OLDEST row's default flag (the one set at provisioning) and unset
//     the rest. Same treatment for the wider (server_id, type) pair to
//     cover future service categories that grow this concept.
//
// The change is idempotent and safe to re-run.
func init() {
	Register(Migration{
		ID:        "0054_05_31_120000_fix_services_is_default",
		Name:      "Flip services.is_default default to false + dedupe per (server_id,type)",
		Timestamp: time.Date(2026, 5, 31, 12, 0, 0, 0, time.UTC),
		Up:        fixServicesIsDefaultUp,
		Down:      fixServicesIsDefaultDown,
	})
}

func fixServicesIsDefaultUp(db *gorm.DB) error {
	// 1. Flip the column default so future inserts that omit
	//    is_default land as false (Postgres + MySQL both accept this
	//    syntax for ALTER COLUMN ... SET DEFAULT).
	if err := db.Exec(`ALTER TABLE services ALTER COLUMN is_default SET DEFAULT FALSE`).Error; err != nil {
		return err
	}

	// 2. Repair existing rows. For every (server_id, type) group where
	//    more than one row is_default=true, keep the row with the
	//    smallest created_at and unset the others.
	//    Using a correlated subquery so this works on both Postgres
	//    and MySQL (no ROW_NUMBER reliance, no Postgres-specific DISTINCT
	//    ON syntax). The id comparison breaks ties deterministically
	//    when two rows share a timestamp.
	return db.Exec(`
		UPDATE services s
		SET is_default = FALSE
		WHERE s.is_default = TRUE
		  AND EXISTS (
		    SELECT 1 FROM services older
		    WHERE older.server_id = s.server_id
		      AND older.type = s.type
		      AND older.is_default = TRUE
		      AND (
		        older.created_at < s.created_at
		        OR (older.created_at = s.created_at AND older.id < s.id)
		      )
		  )
	`).Error
}

func fixServicesIsDefaultDown(db *gorm.DB) error {
	// Restore the original (wrong) default so the migration is
	// reversible. We don't reinstate the duplicate rows — there's no
	// way to know which ones we cleared.
	return db.Exec(`ALTER TABLE services ALTER COLUMN is_default SET DEFAULT TRUE`).Error
}
