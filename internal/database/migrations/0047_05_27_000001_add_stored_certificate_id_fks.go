package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0047_05_27_000001_add_stored_certificate_id_fks",
		Name:      "Add stored_certificate_id FK to certificates and docker_application_domains",
		Timestamp: time.Date(2026, 5, 27, 0, 0, 1, 0, time.UTC),
		Up:        addStoredCertificateFKsUp,
		Down:      addStoredCertificateFKsDown,
	})
}

func addStoredCertificateFKsUp(db *gorm.DB) error {
	if err := db.Exec(`
		ALTER TABLE certificates
		ADD COLUMN stored_certificate_id CHAR(26) NULL
		REFERENCES stored_certificates(id) ON DELETE SET NULL
	`).Error; err != nil {
		return err
	}
	if err := db.Exec(`CREATE INDEX idx_certificates_stored_cert ON certificates (stored_certificate_id)`).Error; err != nil {
		return err
	}

	if err := db.Exec(`
		ALTER TABLE docker_application_domains
		ADD COLUMN stored_certificate_id CHAR(26) NULL
		REFERENCES stored_certificates(id) ON DELETE SET NULL
	`).Error; err != nil {
		return err
	}
	if err := db.Exec(`CREATE INDEX idx_docker_app_domains_stored_cert ON docker_application_domains (stored_certificate_id)`).Error; err != nil {
		return err
	}

	return nil
}

// addStoredCertificateFKsDown removes the FK columns. DROP COLUMN in
// Postgres cascades the FK constraint and the supporting index
// automatically — no manual constraint/index drop needed.
//
// Down ordering note: 0046's Down (DROP TABLE stored_certificates)
// can only run cleanly AFTER this 0047 Down has dropped these FK
// columns. If the migrator ever attempts a "rollback to before
// 0046" while skipping 0047's Down, the DROP TABLE will fail with
// a dependent-objects error. The migrator is expected to walk Downs
// in strict reverse order; this is a one-line reminder for whoever
// reads this migration in isolation.
func addStoredCertificateFKsDown(db *gorm.DB) error {
	if err := db.Exec(`ALTER TABLE certificates DROP COLUMN IF EXISTS stored_certificate_id`).Error; err != nil {
		return err
	}
	if err := db.Exec(`ALTER TABLE docker_application_domains DROP COLUMN IF EXISTS stored_certificate_id`).Error; err != nil {
		return err
	}
	return nil
}
