package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0027_05_23_000002_revamp_docker_database_backups",
		Name:      "Revamp docker_database_backups to use storage_providers FK",
		Timestamp: time.Date(2024, 5, 23, 0, 0, 2, 0, time.UTC),
		Up:        revampDockerDatabaseBackupsUp,
		Down:      revampDockerDatabaseBackupsDown,
	})
}

// revampDockerDatabaseBackupsUp rewires the backup table to reference
// the global storage_providers resource instead of storing S3 creds
// inline. The old shape duplicated bucket/access-key/secret-key per
// database and made the "PHP-site backup" UX (pick a saved storage
// provider from a dropdown) impossible.
//
// Drops:  provider, endpoint, bucket, region, path_prefix, credentials
// Adds:   storage_provider_id (FK → storage_providers.id, uint64)
//
//	path                (varchar 255, bucket sub-folder)
//	retention           (int, number of past runs to keep)
//	notify_on_success   (bool, default false)
//	notify_on_failure   (bool, default true)
//
// Any existing rows are deleted before the schema change — they're
// useless without their storage provider mapping, and we're pre-
// production so there's no real data at stake.
func revampDockerDatabaseBackupsUp(db *gorm.DB) error {
	// Clear out existing rows + their dependent runs. Soft-deletes
	// included so the unique index doesn't trip when the new FK lands.
	if err := db.Exec("DELETE FROM docker_database_backup_runs").Error; err != nil {
		return err
	}
	if err := db.Exec("DELETE FROM docker_database_backups").Error; err != nil {
		return err
	}

	// Drop the old credential-bearing columns. IF EXISTS so the
	// migration is a no-op on fresh installs where the previous
	// shape never existed.
	drops := []string{"provider", "endpoint", "bucket", "region", "path_prefix", "credentials"}
	for _, col := range drops {
		if err := db.Exec(
			"ALTER TABLE docker_database_backups DROP COLUMN IF EXISTS " + col,
		).Error; err != nil {
			return err
		}
	}

	// Add the storage-provider FK column + the new bookkeeping fields.
	// storage_provider_id is BIGINT to match storage_providers.id
	// (autoIncrement uint64).
	adds := []string{
		"ADD COLUMN storage_provider_id BIGINT NOT NULL",
		"ADD COLUMN path VARCHAR(255) NULL",
		"ADD COLUMN retention INT NOT NULL DEFAULT 10",
		"ADD COLUMN notify_on_success BOOLEAN NOT NULL DEFAULT FALSE",
		"ADD COLUMN notify_on_failure BOOLEAN NOT NULL DEFAULT TRUE",
	}
	for _, clause := range adds {
		if err := db.Exec(
			"ALTER TABLE docker_database_backups " + clause,
		).Error; err != nil {
			return err
		}
	}

	// Postgres uses CREATE INDEX as a separate statement (no ADD INDEX inside ALTER TABLE).
	if err := db.Exec(
		"CREATE INDEX idx_docker_db_backups_storage_provider ON docker_database_backups (storage_provider_id)",
	).Error; err != nil {
		return err
	}

	// FK to storage_providers — ON DELETE RESTRICT keeps users from
	// accidentally deleting a provider that's still referenced. The
	// frontend should refuse the delete with a helpful error.
	return db.Exec(
		"ALTER TABLE docker_database_backups " +
			"ADD CONSTRAINT fk_docker_db_backups_storage_provider " +
			"FOREIGN KEY (storage_provider_id) REFERENCES storage_providers(id) " +
			"ON DELETE RESTRICT",
	).Error
}

func revampDockerDatabaseBackupsDown(db *gorm.DB) error {
	// Same nuke-and-pave on the way down so the previous shape is
	// recreated cleanly.
	if err := db.Exec("DELETE FROM docker_database_backup_runs").Error; err != nil {
		return err
	}
	if err := db.Exec("DELETE FROM docker_database_backups").Error; err != nil {
		return err
	}

	if err := db.Exec(
		"ALTER TABLE docker_database_backups DROP CONSTRAINT IF EXISTS fk_docker_db_backups_storage_provider",
	).Error; err != nil {
		return err
	}

	drops := []string{
		"DROP INDEX idx_docker_db_backups_storage_provider",
		"DROP COLUMN IF EXISTS storage_provider_id",
		"DROP COLUMN IF EXISTS path",
		"DROP COLUMN IF EXISTS retention",
		"DROP COLUMN IF EXISTS notify_on_success",
		"DROP COLUMN IF EXISTS notify_on_failure",
	}
	for _, clause := range drops {
		if err := db.Exec(
			"ALTER TABLE docker_database_backups " + clause,
		).Error; err != nil {
			return err
		}
	}

	adds := []string{
		"ADD COLUMN provider VARCHAR(32) NOT NULL DEFAULT 's3'",
		"ADD COLUMN endpoint VARCHAR(255) NULL",
		"ADD COLUMN bucket VARCHAR(255) NOT NULL",
		"ADD COLUMN region VARCHAR(64) NULL",
		"ADD COLUMN path_prefix VARCHAR(255) NULL",
		"ADD COLUMN credentials TEXT NULL",
	}
	for _, clause := range adds {
		if err := db.Exec(
			"ALTER TABLE docker_database_backups " + clause,
		).Error; err != nil {
			return err
		}
	}
	return nil
}
