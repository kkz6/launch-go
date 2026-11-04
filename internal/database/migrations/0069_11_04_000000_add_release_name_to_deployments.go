package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0069_11_04_000000_add_release_name_to_deployments",
		Name:      "Add release name to deployments",
		Timestamp: time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC),
		Up:        addReleaseNameToDeploymentsUp,
	})
}

func addReleaseNameToDeploymentsUp(db *gorm.DB) error {
	if db.Migrator().HasColumn("deployments", "release_name") {
		return nil
	}
	// Legacy names are recovered from their saved task scripts on demand.
	// created_at cannot reconstruct when a queued deployment actually ran.
	return db.Exec("ALTER TABLE deployments ADD COLUMN release_name VARCHAR(64)").Error
}
