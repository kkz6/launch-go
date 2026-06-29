package migrations

import (
	"time"

	"gorm.io/gorm"
)

// Allow empty env var values.
//
// The docker_*_env_vars `value` columns were created NOT NULL, but the
// value is stored as dbtype.EncryptedString whose driver.Valuer returns
// NULL for an empty string (encrypting "" is pointless). So saving a
// legitimately-empty env var — e.g. `SENTRY_DSN=` to disable Sentry —
// tried to INSERT NULL into a NOT NULL column and failed with
// SQLSTATE 23502.
//
// Dropping NOT NULL lets empty values through. The read side already
// round-trips cleanly: EncryptedString.Scan(nil) yields "", so a NULL
// column reads back as an empty string — exactly the value the user
// entered. Nothing queries these columns with `IS NULL`, so making them
// nullable is safe.
//
// DROP NOT NULL on an already-nullable column is a no-op in Postgres, so
// this is naturally idempotent on top of the migrator's run-once
// tracking.
func init() {
	Register(Migration{
		ID:        "0057_06_01_000002_env_var_value_nullable",
		Name:      "Allow empty (NULL) docker env var values",
		Timestamp: time.Date(2026, 6, 1, 0, 0, 2, 0, time.UTC),
		Up:        envVarValueNullableUp,
	})
}

func envVarValueNullableUp(db *gorm.DB) error {
	// One statement per Exec — the Postgres driver auto-prepares each
	// call and rejects multiple semicolon-separated commands (42601).
	statements := []string{
		`ALTER TABLE docker_application_env_vars ALTER COLUMN value DROP NOT NULL`,
		`ALTER TABLE docker_project_env_vars ALTER COLUMN value DROP NOT NULL`,
		`ALTER TABLE docker_database_env_vars ALTER COLUMN value DROP NOT NULL`,
	}
	for _, s := range statements {
		if err := db.Exec(s).Error; err != nil {
			return err
		}
	}
	return nil
}
