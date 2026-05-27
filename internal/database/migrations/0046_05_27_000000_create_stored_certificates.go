package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0046_05_27_000000_create_stored_certificates",
		Name:      "Create stored_certificates table",
		Timestamp: time.Date(2026, 5, 27, 0, 0, 0, 0, time.UTC),
		Up:        createStoredCertificatesUp,
		Down:      createStoredCertificatesDown,
	})
}

// createStoredCertificatesUp lands the team-scoped reusable cert
// library. Same shape registry_credentials / source_controls /
// storage_providers follow: team-scoped, soft-deletable, user_id
// recorded for audit only, `private_key` encrypted at rest via
// dbtype.EncryptedString.
//
// `domains` is JSONB so we can later query "certs covering host X"
// via JSONB containment if needed; dbtype.JSONStringSlice handles
// the GORM scan/value.
//
// Soft-delete uniqueness uses Postgres partial unique indexes
// (WHERE deleted_at IS NULL) — re-using a name after soft-delete is
// allowed because the deleted row is excluded from the index. Same
// pattern applies to the fingerprint dedupe.
func createStoredCertificatesUp(db *gorm.DB) error {
	if err := db.Exec(`
		CREATE TABLE stored_certificates (
			id                  CHAR(26)      NOT NULL PRIMARY KEY,
			team_id             CHAR(26)      NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
			user_id             CHAR(26)      NULL,
			name                VARCHAR(255)  NOT NULL,
			notes               TEXT          NULL,
			certificate         TEXT          NOT NULL,
			private_key         TEXT          NOT NULL,
			domains             JSONB         NOT NULL DEFAULT '[]'::jsonb,
			common_name         VARCHAR(255)  NULL,
			issuer              VARCHAR(255)  NULL,
			not_before          TIMESTAMPTZ   NOT NULL,
			not_after           TIMESTAMPTZ   NOT NULL,
			serial_number       VARCHAR(255)  NULL,
			fingerprint_sha256  VARCHAR(64)   NULL,
			created_at          TIMESTAMPTZ   NULL,
			updated_at          TIMESTAMPTZ   NULL,
			deleted_at          TIMESTAMPTZ   NULL
		)
	`).Error; err != nil {
		return err
	}
	if err := db.Exec(`CREATE INDEX idx_stored_certs_team ON stored_certificates (team_id)`).Error; err != nil {
		return err
	}
	if err := db.Exec(`CREATE INDEX idx_stored_certs_deleted ON stored_certificates (deleted_at)`).Error; err != nil {
		return err
	}
	if err := db.Exec(`
		CREATE UNIQUE INDEX idx_stored_certs_team_name_alive
		ON stored_certificates (team_id, LOWER(name))
		WHERE deleted_at IS NULL
	`).Error; err != nil {
		return err
	}
	if err := db.Exec(`
		CREATE UNIQUE INDEX idx_stored_certs_team_fingerprint_alive
		ON stored_certificates (team_id, fingerprint_sha256)
		WHERE deleted_at IS NULL AND fingerprint_sha256 IS NOT NULL
	`).Error; err != nil {
		return err
	}
	if err := db.Exec(`
		CREATE INDEX idx_stored_certs_expiry
		ON stored_certificates (team_id, not_after)
		WHERE deleted_at IS NULL
	`).Error; err != nil {
		return err
	}
	return nil
}

func createStoredCertificatesDown(db *gorm.DB) error {
	return db.Exec(`DROP TABLE IF EXISTS stored_certificates`).Error
}
