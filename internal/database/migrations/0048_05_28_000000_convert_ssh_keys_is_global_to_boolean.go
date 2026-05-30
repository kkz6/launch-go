package migrations

import (
	"time"

	"gorm.io/gorm"
)

// Fixes a Postgres encoding error on every ssh_keys query that filters
// by is_global:
//
//	failed to encode args[1]: unable to encode true into binary format
//	for int2 (OID 21): cannot find encode plan
//
// The column came over from the legacy MySQL schema as `tinyint(1)`
// (which the PG dump converted to `smallint`). The Go model declares
// IsGlobal as a `bool`, so the pgx driver has no path from Go bool
// to PG int2 and the query fails. Every other tinyint(1) bool in the
// codebase had already been converted to a proper PG boolean during
// the MySQL→PG migration — this one column was missed. Information
// schema confirms it's the only smallint-as-bool column left.
func init() {
	Register(Migration{
		ID:        "0048_05_28_000000_convert_ssh_keys_is_global_to_boolean",
		Name:      "Convert ssh_keys.is_global from smallint to boolean",
		Timestamp: time.Date(2026, 5, 28, 0, 0, 0, 0, time.UTC),
		Up:        convertSshKeysIsGlobalToBooleanUp,
		Down:      convertSshKeysIsGlobalToBooleanDown,
	})
}

func convertSshKeysIsGlobalToBooleanUp(db *gorm.DB) error {
	// Idempotency guard. The `ALTER … TYPE boolean` below auto-commits
	// the moment it runs; if a later statement in this migration ever
	// failed (or the migration row wasn't recorded), the column is left
	// already-boolean and the migration re-runs on the next deploy. On
	// that re-run `is_global <> 0` blows up with
	//   ERROR: operator does not exist: boolean <> integer (SQLSTATE 42883)
	// which crashes the API on boot and fails the Kamal deploy. So only
	// do the type conversion while the column is still the legacy
	// smallint; the steps after it are all idempotent.
	var dataType string
	if err := db.Raw(`
		SELECT data_type FROM information_schema.columns
		WHERE table_name = 'ssh_keys' AND column_name = 'is_global'
	`).Scan(&dataType).Error; err != nil {
		return err
	}

	if dataType != "boolean" {
		// Postgres refuses to alter the column type while the existing
		// `DEFAULT '0'::smallint` is still attached — it can't cast that
		// default to the new boolean type. Drop it first, change the
		// type, then re-attach the boolean default.
		if err := db.Exec(`ALTER TABLE ssh_keys ALTER COLUMN is_global DROP DEFAULT`).Error; err != nil {
			return err
		}
		// USING <expr> tells Postgres how to coerce existing rows. Treat
		// any non-zero smallint as true so the legacy 0/1 encoding maps
		// cleanly onto the new boolean.
		if err := db.Exec(`
			ALTER TABLE ssh_keys
			ALTER COLUMN is_global TYPE boolean
			USING (is_global <> 0)
		`).Error; err != nil {
			return err
		}
	}

	if err := db.Exec(`
		ALTER TABLE ssh_keys
		ALTER COLUMN is_global SET DEFAULT false
	`).Error; err != nil {
		return err
	}
	// The column was nullable in the legacy dump (no NOT NULL was set
	// when the smallint was created). The Go model declares
	// `not null;default:0`, so make the DB match. Backfill any NULL
	// row to false first — there shouldn't be any (the smallint had a
	// '0' default), but defensive.
	if err := db.Exec(`UPDATE ssh_keys SET is_global = false WHERE is_global IS NULL`).Error; err != nil {
		return err
	}
	if err := db.Exec(`ALTER TABLE ssh_keys ALTER COLUMN is_global SET NOT NULL`).Error; err != nil {
		return err
	}
	return nil
}

func convertSshKeysIsGlobalToBooleanDown(db *gorm.DB) error {
	// Reverse: bool → smallint, with default 0, nullable as it was
	// before. Drop the constraint and default before flipping the
	// type — same reason as the Up direction.
	if err := db.Exec(`ALTER TABLE ssh_keys ALTER COLUMN is_global DROP NOT NULL`).Error; err != nil {
		return err
	}
	if err := db.Exec(`ALTER TABLE ssh_keys ALTER COLUMN is_global DROP DEFAULT`).Error; err != nil {
		return err
	}
	if err := db.Exec(`
		ALTER TABLE ssh_keys
		ALTER COLUMN is_global TYPE smallint
		USING CASE WHEN is_global THEN 1 ELSE 0 END
	`).Error; err != nil {
		return err
	}
	if err := db.Exec(`
		ALTER TABLE ssh_keys
		ALTER COLUMN is_global SET DEFAULT 0
	`).Error; err != nil {
		return err
	}
	return nil
}
