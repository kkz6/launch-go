package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0020_02_22_000000_add_provision_error_to_servers",
		Name:      "Add provision_error to servers",
		Timestamp: time.Date(2018, 2, 22, 0, 0, 0, 0, time.UTC),
		Up:        addProvisionErrorToServersUp,
		Down:      addProvisionErrorToServersDown,
	})
}

func addProvisionErrorToServersUp(db *gorm.DB) error {
	// TEXT (no NULL/default needed) — populated by failure callbacks when a
	// provisioning step or cloud-provider create call fails. The UI shows
	// this as a friendly banner instead of a misleading spinner.
	return db.Exec("ALTER TABLE servers ADD COLUMN provision_error TEXT NULL").Error
}

func addProvisionErrorToServersDown(db *gorm.DB) error {
	return db.Exec("ALTER TABLE servers DROP COLUMN provision_error").Error
}
