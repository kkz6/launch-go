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
	return db.Exec(`CREATE INDEX idx_docker_app_domains_stored_cert ON docker_application_domains (stored_certificate_id)`).Error
}
